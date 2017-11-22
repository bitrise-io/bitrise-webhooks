package bitbucketv2

//
// Docs: https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html
//

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/go-utils/sliceutil"
)

const (
	scmGit       = "git"
	scmMercurial = "hg"
)

// --------------------------
// --- Webhook Data Model ---

// ChangeItemTargetModel ...
type ChangeItemTargetModel struct {
	Type          string `json:"type"`
	CommitHash    string `json:"hash"`
	CommitMessage string `json:"message"`
}

// ChangeItemModel ...
type ChangeItemModel struct {
	Type   string                `json:"type"`
	Name   string                `json:"name"`
	Target ChangeItemTargetModel `json:"target"`
}

// ChangeInfoModel ...
type ChangeInfoModel struct {
	ChangeNewItem ChangeItemModel `json:"new"`
}

// PushInfoModel ...
type PushInfoModel struct {
	Changes []ChangeInfoModel `json:"changes"`
}

// PushEventModel ...
type PushEventModel struct {
	PushInfo       PushInfoModel       `json:"push"`
	RepositoryInfo RepositoryInfoModel `json:"repository"`
}

// OwnerInfoModel ...
type OwnerInfoModel struct {
	Username string `json:"username"`
}

// RepositoryInfoModel ...
type RepositoryInfoModel struct {
	FullName  string `json:"full_name"`
	IsPrivate bool   `json:"is_private"`
	// Scm - The type repository: Git (git) or Mercurial (hg).
	Scm   string         `json:"scm"`
	Owner OwnerInfoModel `json:"owner"`
}

// CommitInfoModel ...
type CommitInfoModel struct {
	CommitHash string `json:"hash"`
}

// BranchInfoModel ...
type BranchInfoModel struct {
	Name string `json:"name"`
}

// PullRequestBranchInfoModel ...
type PullRequestBranchInfoModel struct {
	BranchInfo     BranchInfoModel     `json:"branch"`
	CommitInfo     CommitInfoModel     `json:"commit"`
	RepositoryInfo RepositoryInfoModel `json:"repository"`
}

// PullRequestInfoModel ...
type PullRequestInfoModel struct {
	ID              int                        `json:"id"`
	Type            string                     `json:"type"`
	Title           string                     `json:"title"`
	Description     string                     `json:"description"`
	State           string                     `json:"state"`
	SourceInfo      PullRequestBranchInfoModel `json:"source"`
	DestinationInfo PullRequestBranchInfoModel `json:"destination"`
}

// PullRequestEventModel ...
type PullRequestEventModel struct {
	PullRequestInfo PullRequestInfoModel `json:"pullrequest"`
	RepositoryInfo  RepositoryInfoModel  `json:"repository"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentTypeAttemptNumberAndEventKey(header http.Header) (string, string, string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", "", "", errors.New("No Content-Type Header found")
	}

	eventKey := header.Get("X-Event-Key")
	if eventKey == "" {
		return "", "", "", errors.New("No X-Event-Key Header found")
	}

	attemptNum := header.Get("X-Attempt-Number")
	if attemptNum == "" {
		attemptNum = "1"
	}

	return contentType, attemptNum, eventKey, nil
}

func transformPushEvent(pushEvent PushEventModel) hookCommon.TransformResultModel {
	if len(pushEvent.PushInfo.Changes) < 1 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("No 'changes' included in the webhook, can't start a build"),
		}
	}

	switch pushEvent.RepositoryInfo.Scm {
	case scmGit, scmMercurial:
	// supported
	default:
		// unsupported
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported repository / source control type (SCM): %s", pushEvent.RepositoryInfo.Scm),
		}
	}

	triggerAPIParams := []bitriseapi.TriggerAPIParamsModel{}
	errs := []string{}
	for _, aChnage := range pushEvent.PushInfo.Changes {
		aNewItm := aChnage.ChangeNewItem
		if (pushEvent.RepositoryInfo.Scm == scmGit && aNewItm.Type == "branch") ||
			(pushEvent.RepositoryInfo.Scm == scmMercurial && aNewItm.Type == "named_branch") {
			if aNewItm.Target.Type != "commit" {
				errs = append(errs, fmt.Sprintf("Target was not a type=commit change. Type was: %s", aNewItm.Target.Type))
				continue
			}
			aTriggerAPIParams := bitriseapi.TriggerAPIParamsModel{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        aNewItm.Name,
					CommitHash:    aNewItm.Target.CommitHash,
					CommitMessage: aNewItm.Target.CommitMessage,
				},
			}
			triggerAPIParams = append(triggerAPIParams, aTriggerAPIParams)
		} else if aNewItm.Type == "tag" {
			if aNewItm.Target.Type != "commit" {
				errs = append(errs, fmt.Sprintf("Target was not a type=commit change. Type was: %s", aNewItm.Target.Type))
				continue
			}
			aTriggerAPIParams := bitriseapi.TriggerAPIParamsModel{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           aNewItm.Name,
					CommitHash:    aNewItm.Target.CommitHash,
					CommitMessage: aNewItm.Target.CommitMessage,
				},
			}
			triggerAPIParams = append(triggerAPIParams, aTriggerAPIParams)
		} else {
			errs = append(errs, fmt.Sprintf("Not a type=branch nor type=tag change. Change.Type was: %s", aNewItm.Type))
		}
	}
	if len(triggerAPIParams) < 1 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("'changes' specified in the webhook, but none can be transformed into a build. Collected errors: %s", errs),
		}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: triggerAPIParams,
	}
}

func transformPullRequestEvent(pullRequest PullRequestEventModel) hookCommon.TransformResultModel {
	if pullRequest.PullRequestInfo.Type != "pullrequest" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Pull Request type is not supported: %s", pullRequest.PullRequestInfo.Type),
			ShouldSkip: true,
		}
	}

	if pullRequest.PullRequestInfo.State != "OPEN" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Pull Request state doesn't require a build: %s", pullRequest.PullRequestInfo.State),
			ShouldSkip: true,
		}
	}

	commitMsg := pullRequest.PullRequestInfo.Title
	if pullRequest.PullRequestInfo.Description != "" {
		commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, pullRequest.PullRequestInfo.Description)
	}

	if pullRequest.PullRequestInfo.SourceInfo.RepositoryInfo.FullName == pullRequest.PullRequestInfo.DestinationInfo.RepositoryInfo.FullName {
		pullRequest.PullRequestInfo.SourceInfo.RepositoryInfo.IsPrivate = pullRequest.RepositoryInfo.IsPrivate
	} else {
		res, err := http.Head(fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s", pullRequest.PullRequestInfo.SourceInfo.RepositoryInfo.FullName))
		if err != nil {
			return hookCommon.TransformResultModel{
				Error:      fmt.Errorf("Failed to check repository publicity: %s", err),
				ShouldSkip: false,
			}
		}

		pullRequest.PullRequestInfo.SourceInfo.RepositoryInfo.IsPrivate = (res.StatusCode != 200)
	}

	sourceRepositoryURL := ""
	if pullRequest.PullRequestInfo.SourceInfo.RepositoryInfo.IsPrivate {
		sourceRepositoryURL = fmt.Sprintf("git@bitbucket.org:%s.git", pullRequest.PullRequestInfo.SourceInfo.RepositoryInfo.FullName)
	} else {
		sourceRepoFullName := pullRequest.PullRequestInfo.SourceInfo.RepositoryInfo.FullName
		sourceRepositoryURL = fmt.Sprintf("https://bitbucket.org/%s.git", sourceRepoFullName)
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage:            commitMsg,
					CommitHash:               pullRequest.PullRequestInfo.SourceInfo.CommitInfo.CommitHash,
					Branch:                   pullRequest.PullRequestInfo.SourceInfo.BranchInfo.Name,
					BranchDest:               pullRequest.PullRequestInfo.DestinationInfo.BranchInfo.Name,
					PullRequestID:            &pullRequest.PullRequestInfo.ID,
					PullRequestRepositoryURL: sourceRepositoryURL,
				},
			},
		},
	}
}

func isAcceptEventType(eventKey string) bool {
	return sliceutil.IsStringInSlice(eventKey, []string{"repo:push", "pullrequest:created", "pullrequest:updated"})
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(r.Header)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Issue with Headers: %s", err),
		}
	}
	if contentType != hookCommon.ContentTypeApplicationJSON {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if !isAcceptEventType(eventKey) {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("X-Event-Key is not supported: %s", eventKey),
		}
	}
	// Check: is this a re-try hook?
	if attemptNum != "1" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("No retry is supported (X-Attempt-Number: %s)", attemptNum),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	if eventKey == "repo:push" {
		var pushEvent PushEventModel
		if err := json.NewDecoder(r.Body).Decode(&pushEvent); err != nil {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
			}
		}

		return transformPushEvent(pushEvent)
	} else if eventKey == "pullrequest:created" || eventKey == "pullrequest:updated" {
		var pullRequestEvent PullRequestEventModel
		if err := json.NewDecoder(r.Body).Decode(&pullRequestEvent); err != nil {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
			}
		}

		return transformPullRequestEvent(pullRequestEvent)
	}

	return hookCommon.TransformResultModel{
		Error: fmt.Errorf("Unsupported Bitbucket event type: %s", eventKey),
	}
}
