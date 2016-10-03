package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/go-utils/httputil"
	"github.com/bitrise-io/go-utils/sliceutil"
)

// --------------------------
// --- Webhook Data Model ---

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

// RepoInfoModel ...
type RepoInfoModel struct {
	Private bool `json:"private"`
	// Private git clone URL, used with SSH key
	SSHURL string `json:"ssh_url"`
	// Public git clone url
	CloneURL string `json:"clone_url"`
}

// BranchInfoModel ...
type BranchInfoModel struct {
	Ref        string        `json:"ref"`
	CommitHash string        `json:"sha"`
	Repo       RepoInfoModel `json:"repo"`
}

// PullRequestInfoModel ...
type PullRequestInfoModel struct {
	// source brach for the pull request
	HeadBranchInfo BranchInfoModel `json:"head"`
	// destination brach for the pull request
	BaseBranchInfo BranchInfoModel `json:"base"`
	Title          string          `json:"title"`
	Body           string          `json:"body"`
	Merged         bool            `json:"merged"`
	Mergeable      *bool           `json:"mergeable"`
}

// PullRequestEventModel ...
type PullRequestEventModel struct {
	Action          string               `json:"action"`
	PullRequestID   int                  `json:"number"`
	PullRequestInfo PullRequestInfoModel `json:"pull_request"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func transformCodePushEvent(codePushEvent CodePushEventModel) hookCommon.TransformResultModel {
	if codePushEvent.Deleted {
		return hookCommon.TransformResultModel{
			Error:      errors.New("This is a 'Deleted' event, no build can be started"),
			ShouldSkip: true,
		}
	}

	headCommit := codePushEvent.HeadCommit

	if !strings.HasPrefix(codePushEvent.Ref, "refs/heads/") {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Ref (%s) is not a head ref", codePushEvent.Ref),
			ShouldSkip: true,
		}
	}
	branch := strings.TrimPrefix(codePushEvent.Ref, "refs/heads/")

	if len(headCommit.CommitHash) == 0 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Missing commit hash"),
		}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    headCommit.CommitHash,
					CommitMessage: headCommit.CommitMessage,
					Branch:        branch,
				},
			},
		},
	}
}

func isAcceptPullRequestAction(prAction string) bool {
	return sliceutil.IsStringInSlice(prAction, []string{"opened", "reopened", "synchronize", "edited"})
}

func transformPullRequestEvent(pullRequest PullRequestEventModel) hookCommon.TransformResultModel {
	if pullRequest.Action == "" {
		return hookCommon.TransformResultModel{
			Error:      errors.New("No Pull Request action specified"),
			ShouldSkip: true,
		}
	}
	if !isAcceptPullRequestAction(pullRequest.Action) {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Pull Request action doesn't require a build: %s", pullRequest.Action),
			ShouldSkip: true,
		}
	}
	if pullRequest.PullRequestInfo.Merged {
		return hookCommon.TransformResultModel{
			Error:      errors.New("Pull Request already merged"),
			ShouldSkip: true,
		}
	}
	if pullRequest.PullRequestInfo.Mergeable != nil && *pullRequest.PullRequestInfo.Mergeable == false {
		return hookCommon.TransformResultModel{
			Error:      errors.New("Pull Request is not mergeable"),
			ShouldSkip: true,
		}
	}

	commitMsg := pullRequest.PullRequestInfo.Title
	if pullRequest.PullRequestInfo.Body != "" {
		commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, pullRequest.PullRequestInfo.Body)
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage:            commitMsg,
					CommitHash:               pullRequest.PullRequestInfo.HeadBranchInfo.CommitHash,
					Branch:                   pullRequest.PullRequestInfo.HeadBranchInfo.Ref,
					BranchDest:               pullRequest.PullRequestInfo.BaseBranchInfo.Ref,
					PullRequestID:            &pullRequest.PullRequestID,
					PullRequestRepositoryURL: pullRequest.PullRequestInfo.HeadBranchInfo.getRepositoryURL(),
					PullRequestMergeBranch:   fmt.Sprintf("pull/%d/merge", pullRequest.PullRequestID),
				},
			},
		},
	}
}

func detectContentTypeAndEventID(header http.Header) (string, string, error) {
	contentType, err := httputil.GetSingleValueFromHeader("Content-Type", header)
	if err != nil {
		return "", "", fmt.Errorf("Issue with Content-Type Header: %s", err)
	}

	ghEvent, err := httputil.GetSingleValueFromHeader("X-Github-Event", header)
	if err != nil {
		return "", "", fmt.Errorf("Issue with X-Github-Event Header: %s", err)
	}

	return contentType, ghEvent, nil
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, ghEvent, err := detectContentTypeAndEventID(r.Header)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Issue with Headers: %s", err),
		}
	}

	if contentType != hookCommon.ContentTypeApplicationJSON && contentType != hookCommon.ContentTypeApplicationXWWWFormURLEncoded {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if ghEvent == "ping" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Ping event received"),
			ShouldSkip: true,
		}
	}
	if ghEvent != "push" && ghEvent != "pull_request" {
		// Unsupported GitHub Event
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported GitHub Webhook event: %s", ghEvent),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	if ghEvent == "push" {
		// code push
		var codePushEvent CodePushEventModel
		if contentType == hookCommon.ContentTypeApplicationJSON {
			if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
			}
		} else if contentType == hookCommon.ContentTypeApplicationXWWWFormURLEncoded {
			payloadValue := r.PostFormValue("payload")
			if payloadValue == "" {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: empty payload")}
			}
			if err := json.NewDecoder(strings.NewReader(payloadValue)).Decode(&codePushEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse payload: %s", err)}
			}
		} else {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Unsupported Content-Type: %s", contentType),
			}
		}
		return transformCodePushEvent(codePushEvent)

	} else if ghEvent == "pull_request" {
		var pullRequestEvent PullRequestEventModel
		if contentType == hookCommon.ContentTypeApplicationJSON {
			if err := json.NewDecoder(r.Body).Decode(&pullRequestEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body as JSON: %s", err)}
			}
		} else if contentType == hookCommon.ContentTypeApplicationXWWWFormURLEncoded {
			payloadValue := r.PostFormValue("payload")
			if payloadValue == "" {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: empty payload")}
			}
			if err := json.NewDecoder(strings.NewReader(payloadValue)).Decode(&pullRequestEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse payload: %s", err)}
			}
		} else {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Unsupported Content-Type: %s", contentType),
			}
		}
		return transformPullRequestEvent(pullRequestEvent)
	}

	return hookCommon.TransformResultModel{
		Error: fmt.Errorf("Unsupported GitHub event type: %s", ghEvent),
	}
}

// returns the repository clone URL depending on the publicity of the project
func (branchInfoModel BranchInfoModel) getRepositoryURL() string {
	if branchInfoModel.Repo.Private {
		return branchInfoModel.Repo.SSHURL
	}
	return branchInfoModel.Repo.CloneURL
}
