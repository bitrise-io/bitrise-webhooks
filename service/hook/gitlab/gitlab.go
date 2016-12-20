package gitlab

// # Infos / notes:
//
// ## Webhook calls
//
// Official API docs: https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/web_hooks/web_hooks.md
//
// ### Code Push
//
// A code push webhook is sent with the header: `X-Gitlab-Event: Push Hook`.
// Official docs: https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/web_hooks/web_hooks.md#push-events
//
// GitLab sends push webhooks for every branch separately. Even if you
// push to two different branches at the same time (git push --all) it'll
// trigger two webhook calls, one for each branch.
//
// Commits are grouped in the webhook - if you push more than one commit
// to a single branch it'll be included in a single webhook call, including
// all of the commits.
//
// The latest commit's hash is included as the "checkout_sha" parameter
// in the webhook. As we don't want to trigger build for every commit
// which is related to a single branch we will only handle the commit
// with the hash / id specified as the "checkout_sha".
//
// ### Tag Push
//
// GitLab sends webhooks for every tag separately. Even if you create 5 tags and push them with `git push --tags`
// GitLab will send out (properly) 5 separate webhooks, one for every tag (other services typically don't send
// these separately, or don't deliver all tags if you push more than ~3 tags in a single `git push --tags`).
//
// ### Merge request
// A merge request is sent with the header: `X-Gitlab-Event: Merge Request Hook`
// Official docs: https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/web_hooks/web_hooks.md#merge-request-events
//

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/go-utils/sliceutil"
)

// --------------------------
// --- Webhook Data Model ---

const (
	tagPushEventID      = "Tag Push Hook"
	codePushEventID     = "Push Hook"
	mergeRequestEventID = "Merge Request Hook"
)

// CommitModel ...
type CommitModel struct {
	CommitHash    string `json:"id"`
	CommitMessage string `json:"message"`
}

// CodePushEventModel ...
type CodePushEventModel struct {
	ObjectKind  string        `json:"object_kind"`
	Ref         string        `json:"ref"`
	CheckoutSHA string        `json:"checkout_sha"`
	Commits     []CommitModel `json:"commits"`
}

// TagPushEventModel ...
type TagPushEventModel struct {
	ObjectKind  string `json:"object_kind"`
	Ref         string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
}

// BranchInfoModel ...
type BranchInfoModel struct {
	VisibilityLevel int    `json:"visibility_level"`
	GitSSHURL       string `json:"git_ssh_url"`
	GitHTTPURL      string `json:"git_http_url"`
}

// LastCommitInfoModel ...
type LastCommitInfoModel struct {
	SHA string `json:"id"`
}

// ObjectAttributesInfoModel ...
type ObjectAttributesInfoModel struct {
	ID             int                 `json:"iid"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	State          string              `json:"state"`
	MergeCommitSHA string              `json:"merge_commit_sha"`
	MergeError     string              `json:"merge_error"`
	Source         BranchInfoModel     `json:"source"`
	SourceBranch   string              `json:"source_branch"`
	Target         BranchInfoModel     `json:"target"`
	TargetBranch   string              `json:"target_branch"`
	LastCommit     LastCommitInfoModel `json:"last_commit"`
}

// MergeRequestEventModel ...
type MergeRequestEventModel struct {
	ObjectKind       string                    `json:"object_kind"`
	ObjectAttributes ObjectAttributesInfoModel `json:"object_attributes"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentTypeAndEventID(header http.Header) (string, string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", "", errors.New("No Content-Type Header found")
	}

	eventID := header.Get("X-Gitlab-Event")
	if eventID == "" {
		return "", "", errors.New("No X-Gitlab-Event Header found")
	}

	return contentType, eventID, nil
}

func isAcceptEventType(eventKey string) bool {
	return sliceutil.IsStringInSlice(eventKey, []string{tagPushEventID, codePushEventID, mergeRequestEventID})
}

func isAcceptMergeRequestState(prAction string) bool {
	return sliceutil.IsStringInSlice(prAction, []string{"opened", "reopened"})
}

func (branchInfoModel BranchInfoModel) getRepositoryURL() string {
	if branchInfoModel.VisibilityLevel == 20 {
		return branchInfoModel.GitHTTPURL
	}
	return branchInfoModel.GitSSHURL
}

func transformCodePushEvent(codePushEvent CodePushEventModel) hookCommon.TransformResultModel {
	if !strings.HasPrefix(codePushEvent.Ref, "refs/heads/") {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Ref (%s) is not a head ref", codePushEvent.Ref),
			ShouldSkip: true,
		}
	}
	branch := strings.TrimPrefix(codePushEvent.Ref, "refs/heads/")

	lastCommit := CommitModel{}
	isLastCommitFound := false
	for _, aCommit := range codePushEvent.Commits {
		if aCommit.CommitHash == codePushEvent.CheckoutSHA {
			isLastCommitFound = true
			lastCommit = aCommit
			break
		}
	}

	if !isLastCommitFound {
		return hookCommon.TransformResultModel{
			Error: errors.New("The commit specified by 'checkout_sha' was not included in the 'commits' array - no match found"),
		}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    lastCommit.CommitHash,
					CommitMessage: lastCommit.CommitMessage,
					Branch:        branch,
				},
			},
		},
	}
}

func transformTagPushEvent(tagPushEvent TagPushEventModel) hookCommon.TransformResultModel {
	if tagPushEvent.ObjectKind != "tag_push" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Not a Tag Push object: %s", tagPushEvent.ObjectKind),
		}
	}

	if !strings.HasPrefix(tagPushEvent.Ref, "refs/tags/") {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Ref (%s) is not a tags ref", tagPushEvent.Ref),
		}
	}
	tag := strings.TrimPrefix(tagPushEvent.Ref, "refs/tags/")

	if len(tagPushEvent.CheckoutSHA) < 1 {
		return hookCommon.TransformResultModel{
			Error:      errors.New("This is a Tag Deleted event, no build is required"),
			ShouldSkip: true,
		}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        tag,
					CommitHash: tagPushEvent.CheckoutSHA,
				},
			},
		},
	}
}

func transformMergeRequestEvent(mergeRequest MergeRequestEventModel) hookCommon.TransformResultModel {
	if mergeRequest.ObjectKind != "merge_request" {
		return hookCommon.TransformResultModel{
			Error:      errors.New("Not a Merge Request object"),
			ShouldSkip: true,
		}
	}

	if mergeRequest.ObjectAttributes.State == "" {
		return hookCommon.TransformResultModel{
			Error:      errors.New("No Merge Request state specified"),
			ShouldSkip: true,
		}
	}

	if mergeRequest.ObjectAttributes.MergeCommitSHA != "" {
		return hookCommon.TransformResultModel{
			Error:      errors.New("Merge Request already merged"),
			ShouldSkip: true,
		}
	}

	if !isAcceptMergeRequestState(mergeRequest.ObjectAttributes.State) {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Merge Request state doesn't require a build: %s", mergeRequest.ObjectAttributes.State),
			ShouldSkip: true,
		}
	}

	if mergeRequest.ObjectAttributes.MergeError != "" {
		return hookCommon.TransformResultModel{
			Error:      errors.New("Merge Request is not mergeable"),
			ShouldSkip: true,
		}
	}

	commitMsg := mergeRequest.ObjectAttributes.Title
	if mergeRequest.ObjectAttributes.Description != "" {
		commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, mergeRequest.ObjectAttributes.Description)
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage:            commitMsg,
					CommitHash:               mergeRequest.ObjectAttributes.LastCommit.SHA,
					Branch:                   mergeRequest.ObjectAttributes.SourceBranch,
					BranchDest:               mergeRequest.ObjectAttributes.TargetBranch,
					PullRequestID:            &mergeRequest.ObjectAttributes.ID,
					PullRequestRepositoryURL: mergeRequest.ObjectAttributes.Source.getRepositoryURL(),
					PullRequestHeadBranch:    fmt.Sprintf("merge-requests/%d/head", mergeRequest.ObjectAttributes.ID),
				},
			},
		},
	}
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, eventID, err := detectContentTypeAndEventID(r.Header)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Issue with Headers: %s", err),
		}
	}

	if contentType != "application/json" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if !isAcceptEventType(eventID) {
		// Unsupported Event
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported Webhook event: %s", eventID),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	if eventID == codePushEventID {
		// code push
		var codePushEvent CodePushEventModel
		if contentType == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
			}
		}
		return transformCodePushEvent(codePushEvent)
	} else if eventID == tagPushEventID {
		// tag push
		var tagPushEvent TagPushEventModel
		if contentType == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&tagPushEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
			}
		}
		return transformTagPushEvent(tagPushEvent)
	} else if eventID == mergeRequestEventID {
		var mergeRequestEvent MergeRequestEventModel
		if err := json.NewDecoder(r.Body).Decode(&mergeRequestEvent); err != nil {
			return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body as JSON: %s", err)}
		}

		return transformMergeRequestEvent(mergeRequestEvent)
	}

	return hookCommon.TransformResultModel{
		Error: fmt.Errorf("Unsupported GitLab event type: %s", eventID),
	}
}
