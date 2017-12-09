package bitbucketserver

//
// Docs: https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html
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

const (
	scmGit = "git"
)

// --------------------------
// --- Webhook Data Model ---

//PushEventModel ...
type PushEventModel struct {
	EventKey       string              `json:"eventKey"`
	Date           string              `json:"date"`
	Actor          UserInfoModel       `json:"actor"`
	RepositoryInfo RepositoryInfoModel `json:"repository"`
	Changes        []ChangeItemModel   `json:"changes"`
}

//ChangeItemModel ...
type ChangeItemModel struct {
	RefID    string   `json:"refId"`
	FromHash string   `json:"fromHash"`
	ToHash   string   `json:"toHash"`
	Type     string   `json:"type"`
	Ref      RefModel `json:"ref"`
}

//RefModel ...
type RefModel struct {
	ID        string `json:"id"`
	DisplayID string `json:"displayId"`
	Type      string `json:"type"`
}

//UserInfoModel ...
type UserInfoModel struct {
	DisplayName string `json:"displayName"`
}

//RepositoryInfoModel ...
type RepositoryInfoModel struct {
	Slug    string           `json:"slug"`
	ID      int              `json:"id"`
	Name    string           `json:"name"`
	Public  bool             `json:"public"`
	Scm     string           `json:"scmId"`
	Project ProjectInfoModel `json:"owner"`
}

//ProjectInfoModel ...
type ProjectInfoModel struct {
	Key    string `json:"key"`
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
	Type   string `json:"type"`
}

//PullRequestInfoModel ...
type PullRequestInfoModel struct {
	ID          int                 `json:"id"`
	Version     int                 `json:"version"`
	Title       string              `json:"title"`
	State       string              `json:"state"`
	Open        bool                `json:"open"`
	Closed      bool                `json:"closed"`
	CreatedDate int64               `json:"createdDate"`
	UpdatedDate int64               `json:"updatedDate"`
	FromRef     PullRequestRefModel `json:"fromRef"`
	ToRef       PullRequestRefModel `json:"toRef"`
}

//PullRequestEventModel ...
type PullRequestEventModel struct {
	EventKey    string               `json:"eventKey"`
	Date        string               `json:"date"`
	Actor       UserInfoModel        `json:"actor"`
	PullRequest PullRequestInfoModel `json:"pullRequest"`
}

//PullRequestRefModel ...
type PullRequestRefModel struct {
	ID           string              `json:"id"`
	DisplayID    string              `json:"displayId"`
	Type         string              `json:"type"`
	LatestCommit string              `json:"latestCommit"`
	Repository   RepositoryInfoModel `json:"repository"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentTypeSecretAndEventKey(header http.Header) (string, string, string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", "", "", errors.New("No Content-Type Header found")
	}

	eventKey := header.Get("X-Event-Key")
	if eventKey == "" {
		return "", "", "", errors.New("No X-Event-Key Header found")
	}

	secret := header.Get("X-Hub-Signature")

	return contentType, secret, eventKey, nil
}

func transformPushEvent(pushEvent PushEventModel) hookCommon.TransformResultModel {
	if len(pushEvent.Changes) < 1 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("No 'changes' included in the webhook, can't start a build"),
		}
	}

	switch pushEvent.RepositoryInfo.Scm {
	case scmGit:
		// supported
	default:
		// unsupported
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported repository / source control type (SCM): %s", pushEvent.RepositoryInfo.Scm),
		}
	}

	triggerAPIParams := []bitriseapi.TriggerAPIParamsModel{}
	errs := []string{}
	for _, aChange := range pushEvent.Changes {
		if pushEvent.RepositoryInfo.Scm == scmGit && aChange.Type == "UPDATE" {
			if aChange.Ref.Type != "BRANCH" {
				errs = append(errs, fmt.Sprintf("Ref was not a type=BRANCH. Type was: %s", aChange.Ref.Type))
				continue
			}
			aTriggerAPIParams := bitriseapi.TriggerAPIParamsModel{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:     aChange.Ref.DisplayID,
					CommitHash: aChange.ToHash,
				},
			}
			triggerAPIParams = append(triggerAPIParams, aTriggerAPIParams)
		} else if aChange.Type == "ADD" { //tag
			if aChange.Ref.Type != "TAG" {
				errs = append(errs, fmt.Sprintf("Ref was not a type=TAG. Type was: %s", aChange.Ref.Type))
				continue
			}
			aTriggerAPIParams := bitriseapi.TriggerAPIParamsModel{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        aChange.Ref.DisplayID,
					CommitHash: aChange.ToHash,
				},
			}
			triggerAPIParams = append(triggerAPIParams, aTriggerAPIParams)
		} else {
			errs = append(errs, fmt.Sprintf("Not a type=UPDATE nor type=ADD change. Change.Type was: %s", aChange.Type))
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

	if pullRequest.PullRequest.State != "OPEN" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Pull Request state doesn't require a build: %s", pullRequest.PullRequest.State),
			ShouldSkip: true,
		}
	}

	commitMsg := pullRequest.PullRequest.Title

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage: commitMsg,
					CommitHash:    pullRequest.PullRequest.FromRef.LatestCommit,
					Branch:        pullRequest.PullRequest.FromRef.DisplayID,
					BranchDest:    pullRequest.PullRequest.ToRef.DisplayID,
					PullRequestID: &pullRequest.PullRequest.ID,
				},
			},
		},
	}
}

func isAcceptEventType(eventKey string) bool {
	return sliceutil.IsStringInSlice(eventKey, []string{"repo:refs_changed", "pr:opened"})
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, secret, eventKey, err := detectContentTypeSecretAndEventKey(r.Header)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Issue with Headers: %s", err),
		}
	}
	if !strings.HasPrefix(contentType, hookCommon.ContentTypeApplicationJSON) {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if !isAcceptEventType(eventKey) {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("X-Event-Key is not supported: %s", eventKey),
		}
	}
	if secret != "" {
		// todo handle secret
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	if eventKey == "repo:refs_changed" {
		var pushEvent PushEventModel
		if err := json.NewDecoder(r.Body).Decode(&pushEvent); err != nil {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
			}
		}

		return transformPushEvent(pushEvent)
	} else if eventKey == "pr:opened" {
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
