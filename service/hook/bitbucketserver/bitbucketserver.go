package bitbucketserver

//
// Docs: https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html
//

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

const (
	scmGit        = "git"
	actionAdd     = "ADD"
	actionUpdate  = "UPDATE"
	refTypeBranch = "BRANCH"
	refTypeTag    = "TAG"

	// ProviderID ...
	ProviderID = "bitbucket-server"
)

// --------------------------
// --- Webhook Data Model ---

// PushEventModel ...
type PushEventModel struct {
	EventKey       string              `json:"eventKey"`
	Date           string              `json:"date"`
	Actor          UserInfoModel       `json:"actor"`
	RepositoryInfo RepositoryInfoModel `json:"repository"`
	Changes        []ChangeItemModel   `json:"changes"`
	Commits        []CommitModel       `json:"commits"`
}

// ChangeItemModel ...
type ChangeItemModel struct {
	RefID    string   `json:"refId"`
	FromHash string   `json:"fromHash"`
	ToHash   string   `json:"toHash"`
	Type     string   `json:"type"`
	Ref      RefModel `json:"ref"`
}

// RefModel ...
type RefModel struct {
	ID        string `json:"id"`
	DisplayID string `json:"displayId"`
	Type      string `json:"type"`
}

// UserInfoModel ...
type UserInfoModel struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// RepositoryInfoModel ...
type RepositoryInfoModel struct {
	Slug    string           `json:"slug"`
	ID      int              `json:"id"`
	Name    string           `json:"name"`
	Public  bool             `json:"public"`
	Scm     string           `json:"scmId"`
	Project ProjectInfoModel `json:"project"`
}

// CommitModel ...
type CommitModel struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// ProjectInfoModel ...
type ProjectInfoModel struct {
	Key    string `json:"key"`
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
	Type   string `json:"type"`
}

// PullRequestInfoModel ...
type PullRequestInfoModel struct {
	ID          int                 `json:"id"`
	Version     int                 `json:"version"`
	Title       string              `json:"title"`
	State       string              `json:"state"`
	Open        bool                `json:"open"`
	Closed      bool                `json:"closed"`
	CreatedDate int64               `json:"createdDate"`
	UpdatedDate int64               `json:"updatedDate"`
	Author      AuthorModel         `json:"author"`
	FromRef     PullRequestRefModel `json:"fromRef"`
	ToRef       PullRequestRefModel `json:"toRef"`
}

// PullRequestEventModel ...
type PullRequestEventModel struct {
	EventKey    string               `json:"eventKey"`
	Date        string               `json:"date"`
	Actor       UserInfoModel        `json:"actor"`
	PullRequest PullRequestInfoModel `json:"pullRequest"`
	CommentInfo *CommentModel        `json:"comment"`
}

// PullRequestRefModel ...
type PullRequestRefModel struct {
	ID           string              `json:"id"`
	DisplayID    string              `json:"displayId"`
	LatestCommit string              `json:"latestCommit"`
	Repository   RepositoryInfoModel `json:"repository"`
}

// CommentModel ...
type CommentModel struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// AuthorModel
type AuthorModel struct {
	User UserInfoModel `json:"user"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct {
	timeProvider hookCommon.TimeProvider
}

// NewHookProvider ...
func NewHookProvider(timeProvider hookCommon.TimeProvider) hookCommon.Provider {
	return HookProvider{
		timeProvider: timeProvider,
	}
}

// NewDefaultHookProvider ...
func NewDefaultHookProvider() hookCommon.Provider {
	return NewHookProvider(hookCommon.NewDefaultTimeProvider())
}

func detectContentTypeAndEventKey(header http.Header) (string, string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", "", errors.New("No Content-Type Header found")
	}

	eventKey := header.Get("X-Event-Key")
	if eventKey == "" {
		return "", "", errors.New("No X-Event-Key Header found")
	}

	return contentType, eventKey, nil
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
	var errs []string

	var validChangeCount = 0
	for _, change := range pushEvent.Changes {
		if change.Ref.Type == refTypeBranch && (change.Type == actionAdd || change.Type == actionUpdate) {
			validChangeCount++
		} else if change.Ref.Type == refTypeTag && change.Type == actionAdd {
			validChangeCount++
		}
	}
	multipleChanges := validChangeCount > 1

	var commitMessages = make(map[string]string)
	var allCommitMessages []string
	if !multipleChanges {
		for _, commit := range pushEvent.Commits {
			commitMessages[commit.ID] = commit.Message
			allCommitMessages = append(allCommitMessages, commit.Message)
		}
	}

	for _, aChange := range pushEvent.Changes {
		if pushEvent.RepositoryInfo.Scm == scmGit && aChange.Ref.Type == refTypeBranch {
			if aChange.Type != actionAdd && aChange.Type != actionUpdate {
				errs = append(errs, fmt.Sprintf("Not a type=UPDATE nor type=ADD change. Change.Type was: %s", aChange.Type))
				continue
			}

			headCommmitMessage, _ := commitMessages[aChange.ToHash]

			aTriggerAPIParams := bitriseapi.TriggerAPIParamsModel{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:         aChange.Ref.DisplayID,
					CommitHash:     aChange.ToHash,
					CommitMessage:  headCommmitMessage,
					CommitMessages: allCommitMessages,
				},
				TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, pushEvent.Actor.Name),
			}
			triggerAPIParams = append(triggerAPIParams, aTriggerAPIParams)
		} else if aChange.Ref.Type == refTypeTag { //tag
			if aChange.Type != actionAdd {
				errs = append(errs, fmt.Sprintf("Not a type=ADD change. Change.Type was: %s", aChange.Type))
				continue
			}

			headCommmitMessage, _ := commitMessages[aChange.ToHash]

			aTriggerAPIParams := bitriseapi.TriggerAPIParamsModel{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:            aChange.Ref.DisplayID,
					CommitHash:     aChange.ToHash,
					CommitMessage:  headCommmitMessage,
					CommitMessages: allCommitMessages,
				},
				TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, pushEvent.Actor.Name),
			}
			triggerAPIParams = append(triggerAPIParams, aTriggerAPIParams)
		} else {
			errs = append(errs, fmt.Sprintf("Ref was not a type=BRANCH nor type=TAG change. Type was: %s", aChange.Ref.Type))
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
	// Note that description is missing here

	var comment string
	var commentID string
	if pullRequest.CommentInfo != nil {
		comment = pullRequest.CommentInfo.Text
		commentID = strconv.Itoa(pullRequest.CommentInfo.ID)
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitMessage:        commitMsg,
					CommitHash:           pullRequest.PullRequest.FromRef.LatestCommit,
					Branch:               pullRequest.PullRequest.FromRef.DisplayID,
					BranchRepoOwner:      pullRequest.PullRequest.FromRef.Repository.Project.Key,
					BranchDest:           pullRequest.PullRequest.ToRef.DisplayID,
					BranchDestRepoOwner:  pullRequest.PullRequest.ToRef.Repository.Project.Key,
					PullRequestID:        &pullRequest.PullRequest.ID,
					PullRequestAuthor:    pullRequest.PullRequest.Author.User.Name,
					PullRequestComment:   comment,
					PullRequestCommentID: commentID,
				},
				TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, pullRequest.Actor.Name),
			},
		},
	}
}

func isAcceptEventType(eventKey string) bool {
	return slices.Contains([]string{"repo:refs_changed", "pr:opened", "pr:modified", "pr:merged", "diagnostics:ping", "pr:from_ref_updated", "pr:comment:added", "pr:comment:edited"}, eventKey)
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, eventKey, err := detectContentTypeAndEventKey(r.Header)
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
	}

	if eventKey == "pr:opened" || eventKey == "pr:modified" || eventKey == "pr:merged" || eventKey == "pr:from_ref_updated" || eventKey == "pr:comment:added" || eventKey == "pr:comment:edited" {
		var pullRequestEvent PullRequestEventModel
		if err := json.NewDecoder(r.Body).Decode(&pullRequestEvent); err != nil {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
			}
		}

		return transformPullRequestEvent(pullRequestEvent)
	}

	if eventKey == "diagnostics:ping" {
		return hookCommon.TransformResultModel{
			ShouldSkip: true,
			Error:      fmt.Errorf("Bitbucket event type: %s is successful", eventKey),
		}
	}

	return hookCommon.TransformResultModel{
		Error: fmt.Errorf("Unsupported Bitbucket event type: %s", eventKey),
	}
}
