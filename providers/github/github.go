package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/bitrise-webhooks/providers"
	"github.com/bitrise-io/go-utils/httputil"
	"github.com/bitrise-io/go-utils/sliceutil"
)

// CommitModel ...
type CommitModel struct {
	Distinct      bool   `json:"distinct"`
	CommitHash    string `json:"id"`
	CommitMessage string `json:"message"`
}

// CodePushEventModel ...
type CodePushEventModel struct {
	Ref        string      `json:"ref"`
	Deleted    bool        `json:"deleted"`
	HeadCommit CommitModel `json:"head_commit"`
}

// BranchInfoModel ...
type BranchInfoModel struct {
	Ref        string `json:"ref"`
	CommitHash string `json:"sha"`
}

// PullRequestInfoModel ...
type PullRequestInfoModel struct {
	BranchInfo BranchInfoModel `json:"head"`
	Title      string          `json:"title"`
	Body       string          `json:"body"`
	Merged     bool            `json:"merged"`
	Mergeable  *bool           `json:"mergeable"`
}

// PullRequestEventModel ...
type PullRequestEventModel struct {
	Action          string               `json:"action"`
	PullRequestID   int                  `json:"number"`
	PullRequestInfo PullRequestInfoModel `json:"pull_request"`
}

// HookProvider ...
type HookProvider struct{}

// HookCheck ...
func (hp HookProvider) HookCheck(header http.Header) providers.HookCheckModel {
	if contentType, err := httputil.GetSingleValueFromHeader("Content-Type", header); err != nil {
		return providers.HookCheckModel{IsSupportedByProvider: false}
	} else if contentType != "application/json" && contentType != "application/x-www-form-urlencoded" {
		return providers.HookCheckModel{IsSupportedByProvider: false}
	}

	ghEvent, err := httputil.GetSingleValueFromHeader("X-Github-Event", header)
	if err != nil {
		return providers.HookCheckModel{IsSupportedByProvider: false}
	}

	if ghEvent == "push" || ghEvent == "pull_request" {
		// We'll process this
		return providers.HookCheckModel{IsSupportedByProvider: true}
	}

	// GitHub webhook, but not supported event type - skip it
	return providers.HookCheckModel{
		IsSupportedByProvider: true,
		CantTransformReason:   fmt.Errorf("Unsupported GitHub hook event type: %s", ghEvent),
	}
}

func transformCodePushEvent(codePushEvent CodePushEventModel) providers.HookTransformResultModel {
	if codePushEvent.Deleted {
		return providers.HookTransformResultModel{
			Error:      errors.New("This is a 'Deleted' event, no build can be started"),
			ShouldSkip: true,
		}
	}

	headCommit := codePushEvent.HeadCommit
	if !headCommit.Distinct {
		return providers.HookTransformResultModel{
			Error:      errors.New("Head Commit is not Distinct"),
			ShouldSkip: true,
		}
	}

	if !strings.HasPrefix(codePushEvent.Ref, "refs/heads/") {
		return providers.HookTransformResultModel{
			Error:      fmt.Errorf("Ref (%s) is not a head ref", codePushEvent.Ref),
			ShouldSkip: true,
		}
	}
	branch := strings.TrimPrefix(codePushEvent.Ref, "refs/heads/")

	return providers.HookTransformResultModel{
		TriggerAPIParams: bitriseapi.TriggerAPIParamsModel{
			CommitHash:    headCommit.CommitHash,
			CommitMessage: headCommit.CommitMessage,
			Branch:        branch,
		},
	}
}

func transformPullRequestEvent(pullRequest PullRequestEventModel) providers.HookTransformResultModel {
	if pullRequest.Action == "" {
		return providers.HookTransformResultModel{
			Error:      errors.New("No Pull Request action specified"),
			ShouldSkip: true,
		}
	}
	if !sliceutil.IsStringInSlice(pullRequest.Action, []string{"opened", "reopened", "synchronize"}) {
		return providers.HookTransformResultModel{
			Error:      fmt.Errorf("Pull Request action doesn't require a build: %s", pullRequest.Action),
			ShouldSkip: true,
		}
	}
	if pullRequest.PullRequestInfo.Merged {
		return providers.HookTransformResultModel{
			Error:      errors.New("Pull Request already merged"),
			ShouldSkip: true,
		}
	}
	if pullRequest.PullRequestInfo.Mergeable != nil && *pullRequest.PullRequestInfo.Mergeable == false {
		return providers.HookTransformResultModel{
			Error:      errors.New("Pull Request is not mergeable"),
			ShouldSkip: true,
		}
	}

	commitMsg := pullRequest.PullRequestInfo.Title
	if pullRequest.PullRequestInfo.Body != "" {
		commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, pullRequest.PullRequestInfo.Body)
	}

	return providers.HookTransformResultModel{
		TriggerAPIParams: bitriseapi.TriggerAPIParamsModel{
			CommitHash:    pullRequest.PullRequestInfo.BranchInfo.CommitHash,
			CommitMessage: commitMsg,
			Branch:        pullRequest.PullRequestInfo.BranchInfo.Ref,
			PullRequestID: &pullRequest.PullRequestID,
		},
	}
}

// Transform ...
func (hp HookProvider) Transform(r *http.Request) providers.HookTransformResultModel {
	if r.Body == nil {
		return providers.HookTransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	contentType, err := httputil.GetSingleValueFromHeader("Content-Type", r.Header)
	if err != nil {
		return providers.HookTransformResultModel{
			Error: fmt.Errorf("Failed to get Content-Type from Header"),
		}
	}

	ghEvent, err := httputil.GetSingleValueFromHeader("X-Github-Event", r.Header)
	if err != nil {
		return providers.HookTransformResultModel{
			Error: fmt.Errorf("Failed to get Github-Event from Header"),
		}
	}

	if ghEvent == "push" {
		// code push
		var codePushEvent CodePushEventModel
		if contentType == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
			}
		} else if contentType == "application/x-www-form-urlencoded" {
			payloadValue := r.PostFormValue("payload")
			if payloadValue == "" {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse request body: empty payload")}
			}
			if err := json.NewDecoder(strings.NewReader(payloadValue)).Decode(&codePushEvent); err != nil {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse payload: %s", err)}
			}
		} else {
			return providers.HookTransformResultModel{
				Error: fmt.Errorf("Unsupported Content-Type: %s", contentType),
			}
		}
		return transformCodePushEvent(codePushEvent)

	} else if ghEvent == "pull_request" {
		var pullRequestEvent PullRequestEventModel
		if contentType == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&pullRequestEvent); err != nil {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
			}
		} else if contentType == "application/x-www-form-urlencoded" {
			payloadValue := r.PostFormValue("payload")
			if payloadValue == "" {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse request body: empty payload")}
			}
			if err := json.NewDecoder(strings.NewReader(payloadValue)).Decode(&pullRequestEvent); err != nil {
				return providers.HookTransformResultModel{Error: fmt.Errorf("Failed to parse payload: %s", err)}
			}
		} else {
			return providers.HookTransformResultModel{
				Error: fmt.Errorf("Unsupported Content-Type: %s", contentType),
			}
		}
		return transformPullRequestEvent(pullRequestEvent)
	}

	return providers.HookTransformResultModel{
		Error: fmt.Errorf("Unsupported GitHub event type: %s", ghEvent),
	}
}
