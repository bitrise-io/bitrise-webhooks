package github

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

	// ProviderID ...
	ProviderID = "github"
)

// --------------------------
// --- Webhook Data Model ---

// CommitModel ...
type CommitModel struct {
	bitriseapi.CommitPaths
	Distinct      bool   `json:"distinct"`
	CommitHash    string `json:"id"`
	CommitMessage string `json:"message"`
}

// PushEventModel ...
type PushEventModel struct {
	Ref        string        `json:"ref"`
	Deleted    bool          `json:"deleted"`
	HeadCommit CommitModel   `json:"head_commit"`
	Commits    []CommitModel `json:"commits"`
	Repo       RepoInfoModel `json:"repository"`
	Pusher     PusherModel   `json:"pusher"`
}

// UserModel ...
type UserModel struct {
	Login string `json:"login"`
}

// PusherModel ...
type PusherModel struct {
	Name string `json:"name"`
}

// RepoInfoModel ...
type RepoInfoModel struct {
	Private bool `json:"private"`
	// Private git clone URL, used with SSH key
	SSHURL string `json:"ssh_url"`
	// Public git clone url
	CloneURL string `json:"clone_url"`
	// Owner information
	Owner UserModel `json:"owner"`
}

// BranchInfoModel ...
type BranchInfoModel struct {
	Ref        string        `json:"ref"`
	CommitHash string        `json:"sha"`
	Repo       RepoInfoModel `json:"repo"`
}

// LabelInfoModel ...
type LabelInfoModel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// PullRequestInfoModel ...
type PullRequestInfoModel struct {
	// source branch for the pull request
	HeadBranchInfo BranchInfoModel `json:"head"`
	// destination branch for the pull request
	BaseBranchInfo BranchInfoModel  `json:"base"`
	Title          string           `json:"title"`
	Body           string           `json:"body"`
	Merged         bool             `json:"merged"`
	Mergeable      *bool            `json:"mergeable"`
	Draft          bool             `json:"draft"`
	DiffURL        string           `json:"diff_url"`
	User           UserModel        `json:"user"`
	Labels         []LabelInfoModel `json:"labels"`
}

// PullRequestChangeFromItemModel ...
type PullRequestChangeFromItemModel struct {
	From string `json:"from"`
}

// PullRequestChangesInfoModel ...
type PullRequestChangesInfoModel struct {
	Title PullRequestChangeFromItemModel `json:"title"`
	Body  PullRequestChangeFromItemModel `json:"body"`
	Base  interface{}                    `json:"base"`
}

// PullRequestEventModel ...
type PullRequestEventModel struct {
	Action          string                      `json:"action"`
	PullRequestID   int                         `json:"number"`
	PullRequestInfo PullRequestInfoModel        `json:"pull_request"`
	Changes         PullRequestChangesInfoModel `json:"changes"`
	Label           *LabelInfoModel             `json:"label"`
	Sender          UserModel                   `json:"sender"`
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

func transformPushEvent(pushEvent PushEventModel) hookCommon.TransformResultModel {
	if pushEvent.Deleted {
		return hookCommon.TransformResultModel{
			Error: errors.New("this is a 'Deleted' event, no build can be started"),
			// ShouldSkip because there's no reason to respond with a "red" / 4xx error for this event,
			// but this event should never start a build either, so we mark this with `ShouldSkip`
			// to return with the error message (above), but with a "green" / 2xx http code.
			ShouldSkip: true,
		}
	}

	if !strings.HasPrefix(pushEvent.Ref, "refs/heads/") && !strings.HasPrefix(pushEvent.Ref, "refs/tags/") {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("ref (%s) is not a head nor a tag ref", pushEvent.Ref),
			ShouldSkip: true,
		}
	}

	headCommit := pushEvent.HeadCommit
	if len(headCommit.CommitHash) == 0 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("missing commit hash"),
		}
	}

	var commits = pushEvent.Commits
	if len(commits) == 0 {
		commits = []CommitModel{pushEvent.HeadCommit}
	}

	var commitPaths []bitriseapi.CommitPaths
	var commitMessages []string
	for _, commit := range commits {
		commitPaths = append(commitPaths, commit.CommitPaths)
		commitMessages = append(commitMessages, commit.CommitMessage)
	}

	if strings.HasPrefix(pushEvent.Ref, "refs/heads/") {
		// code push
		branch := strings.TrimPrefix(pushEvent.Ref, "refs/heads/")

		return hookCommon.TransformResultModel{
			TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Branch:            branch,
						CommitHash:        headCommit.CommitHash,
						CommitMessage:     headCommit.CommitMessage,
						CommitMessages:    commitMessages,
						PushCommitPaths:   commitPaths,
						BaseRepositoryURL: pushEvent.Repo.getRepositoryURL(),
					},
					TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, pushEvent.Pusher.Name),
				},
			},
		}
	} else if strings.HasPrefix(pushEvent.Ref, "refs/tags/") {
		// tag push
		tag := strings.TrimPrefix(pushEvent.Ref, "refs/tags/")

		return hookCommon.TransformResultModel{
			TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Tag:               tag,
						CommitHash:        headCommit.CommitHash,
						CommitMessage:     headCommit.CommitMessage,
						CommitMessages:    commitMessages,
						PushCommitPaths:   commitPaths,
						BaseRepositoryURL: pushEvent.Repo.getRepositoryURL(),
					},
					TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, pushEvent.Pusher.Name),
				},
			},
		}
	}

	return hookCommon.TransformResultModel{}
}

func isAcceptPullRequestAction(prAction string) bool {
	return slices.Contains([]string{"opened", "reopened", "synchronize", "edited", "ready_for_review", "labeled"}, prAction)
}

func transformPullRequestEvent(pullRequest PullRequestEventModel) hookCommon.TransformResultModel {
	if pullRequest.Action == "" {
		return hookCommon.TransformResultModel{
			Error:      errors.New("no Pull Request action specified"),
			ShouldSkip: true,
		}
	}
	if !isAcceptPullRequestAction(pullRequest.Action) {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("pull Request action doesn't require a build: %s", pullRequest.Action),
			ShouldSkip: true,
		}
	}
	if pullRequest.Action == "edited" {
		// skip it if only title / description changed, and the previous pattern did not include a [skip ci] pattern
		if pullRequest.Changes.Base == nil {
			if !hookCommon.IsSkipBuildByCommitMessage(pullRequest.Changes.Title.From) && !hookCommon.IsSkipBuildByCommitMessage(pullRequest.Changes.Body.From) {
				return hookCommon.TransformResultModel{
					Error:      errors.New("pull Request edit doesn't require a build: only title and/or description was changed, and previous one was not skipped"),
					ShouldSkip: true,
				}
			}
		}
	}
	if pullRequest.PullRequestInfo.Merged {
		return hookCommon.TransformResultModel{
			Error:      errors.New("pull Request already merged"),
			ShouldSkip: true,
		}
	}
	if pullRequest.Action == "labeled" && pullRequest.PullRequestInfo.Mergeable == nil {
		return hookCommon.TransformResultModel{
			Error:      errors.New("pull Request label added to PR that is not open yet"),
			ShouldSkip: true,
		}
	}

	headRefBuildParam := fmt.Sprintf("pull/%d/head", pullRequest.PullRequestID)
	unverifiedMergeRefBuildParam := fmt.Sprintf("pull/%d/merge", pullRequest.PullRequestID)
	// If `mergeable` is nil, the merge ref is not up-to-date, it's not safe to use for checkouts.
	mergeRefUpToDate := pullRequest.PullRequestInfo.Mergeable != nil
	var mergeRefBuildParam string
	if mergeRefUpToDate {
		mergeRefBuildParam = unverifiedMergeRefBuildParam
	}
	if mergeRefUpToDate && *pullRequest.PullRequestInfo.Mergeable == false {
		return hookCommon.TransformResultModel{
			Error:      errors.New("pull Request is not mergeable"),
			ShouldSkip: true,
		}
	}

	commitMsg := pullRequest.PullRequestInfo.Title
	if pullRequest.PullRequestInfo.Body != "" {
		commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, pullRequest.PullRequestInfo.Body)
	}

	buildEnvs := make([]bitriseapi.EnvironmentItem, 0)
	if pullRequest.PullRequestInfo.Draft {
		buildEnvs = append(buildEnvs, bitriseapi.EnvironmentItem{
			Name:     "GITHUB_PR_IS_DRAFT",
			Value:    strconv.FormatBool(pullRequest.PullRequestInfo.Draft),
			IsExpand: false,
		})
	}

	var labels []string
	for _, label := range pullRequest.PullRequestInfo.Labels {
		labels = append(labels, label.Name)
	}

	result := bitriseapi.TriggerAPIParamsModel{
		BuildParams: bitriseapi.BuildParamsModel{
			CommitMessage:                    commitMsg,
			CommitHash:                       pullRequest.PullRequestInfo.HeadBranchInfo.CommitHash,
			Branch:                           pullRequest.PullRequestInfo.HeadBranchInfo.Ref,
			BranchRepoOwner:                  pullRequest.PullRequestInfo.HeadBranchInfo.Repo.Owner.Login,
			BranchDest:                       pullRequest.PullRequestInfo.BaseBranchInfo.Ref,
			BranchDestRepoOwner:              pullRequest.PullRequestInfo.BaseBranchInfo.Repo.Owner.Login,
			PullRequestID:                    &pullRequest.PullRequestID,
			BaseRepositoryURL:                pullRequest.PullRequestInfo.BaseBranchInfo.getRepositoryURL(),
			HeadRepositoryURL:                pullRequest.PullRequestInfo.HeadBranchInfo.getRepositoryURL(),
			PullRequestRepositoryURL:         pullRequest.PullRequestInfo.HeadBranchInfo.getRepositoryURL(),
			PullRequestAuthor:                pullRequest.PullRequestInfo.User.Login,
			PullRequestHeadBranch:            headRefBuildParam,
			PullRequestMergeBranch:           mergeRefBuildParam,
			PullRequestUnverifiedMergeBranch: unverifiedMergeRefBuildParam,
			DiffURL:                          pullRequest.PullRequestInfo.DiffURL,
			Environments:                     buildEnvs,
			PullRequestReadyState:            pullRequestReadyState(pullRequest),
			PullRequestLabels:                labels,
		},
		TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, pullRequest.Sender.Login),
	}

	if pullRequest.Label != nil {
		result.BuildParams.PullRequestLabelsAdded = []string{pullRequest.Label.Name}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			result,
		},
		SkippedByPrDescription: !hookCommon.IsSkipBuildByCommitMessage(pullRequest.PullRequestInfo.Title) &&
			hookCommon.IsSkipBuildByCommitMessage(pullRequest.PullRequestInfo.Body),
	}
}

func pullRequestReadyState(pullRequest PullRequestEventModel) bitriseapi.PullRequestReadyState {
	switch {
	case pullRequest.Action == "ready_for_review":
		return bitriseapi.PullRequestReadyStateConvertedToReadyForReview
	case pullRequest.PullRequestInfo.Draft:
		return bitriseapi.PullRequestReadyStateDraft
	default:
		return bitriseapi.PullRequestReadyStateReadyForReview
	}
}

func detectContentTypeAndEventID(header http.Header) (string, string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", "", errors.New("No Content-Type Header found")
	}

	ghEvent := header.Get("X-Github-Event")
	if ghEvent == "" {
		return "", "", errors.New("No X-Github-Event Header found")
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
			Error:      fmt.Errorf("ping event received"),
			ShouldSkip: true,
		}
	}
	if ghEvent != "push" && ghEvent != "pull_request" {
		// Unsupported GitHub Event
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("unsupported GitHub Webhook event: %s", ghEvent),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("failed to read content of request body: no or empty request body"),
		}
	}

	if ghEvent == "push" {
		// push (code & tag)
		var pushEvent PushEventModel
		if contentType == hookCommon.ContentTypeApplicationJSON {
			if err := json.NewDecoder(r.Body).Decode(&pushEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
			}
		} else if contentType == hookCommon.ContentTypeApplicationXWWWFormURLEncoded {
			payloadValue := r.PostFormValue("payload")
			if payloadValue == "" {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("failed to parse request body: empty payload")}
			}
			if err := json.NewDecoder(strings.NewReader(payloadValue)).Decode(&pushEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse payload: %s", err)}
			}
		} else {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Unsupported Content-Type: %s", contentType),
			}
		}
		return transformPushEvent(pushEvent)

	} else if ghEvent == "pull_request" {
		var pullRequestEvent PullRequestEventModel
		if contentType == hookCommon.ContentTypeApplicationJSON {
			if err := json.NewDecoder(r.Body).Decode(&pullRequestEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body as JSON: %s", err)}
			}
		} else if contentType == hookCommon.ContentTypeApplicationXWWWFormURLEncoded {
			payloadValue := r.PostFormValue("payload")
			if payloadValue == "" {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("failed to parse request body: empty payload")}
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

func (branchInfoModel BranchInfoModel) getRepositoryURL() string {
	return branchInfoModel.Repo.getRepositoryURL()
}

func (repoInfoModel RepoInfoModel) getRepositoryURL() string {
	if repoInfoModel.Private {
		return repoInfoModel.SSHURL
	}
	return repoInfoModel.CloneURL
}
