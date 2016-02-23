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

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/go-utils/httputil"
)

// --------------------------
// --- Webhook Data Model ---

const (
	pushEventID = "Push Hook"
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

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentTypeAndEventID(header http.Header) (string, string, error) {
	contentType, err := httputil.GetSingleValueFromHeader("Content-Type", header)
	if err != nil {
		return "", "", fmt.Errorf("Issue with Content-Type Header: %s", err)
	}

	eventID, err := httputil.GetSingleValueFromHeader("X-Gitlab-Event", header)
	if err != nil {
		return "", "", fmt.Errorf("Issue with X-Gitlab-Event Header: %s", err)
	}

	return contentType, eventID, nil
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

	if eventID != "Push Hook" {
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

	// code push
	var codePushEvent CodePushEventModel
	if contentType == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
			return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
		}
	}
	return transformCodePushEvent(codePushEvent)
}
