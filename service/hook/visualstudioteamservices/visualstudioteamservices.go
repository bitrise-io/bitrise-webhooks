package visualstudioteamservices

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

const (
	emptyCommitHash = "0000000000000000000000000000000000000000"

	// ProviderID ...
	ProviderID = "visualstudio"

	// Push event name
	Push string = "git.push"
	// PullRequestCreate event name
	PullRequestCreate = "git.pullrequest.created"
	// PullRequestUpdate event name
	PullRequestUpdate = "git.pullrequest.updated"
)

// --------------------------
// --- Webhook Data Model ---

// CommitModel ...
type CommitModel struct {
	CommitID string `json:"commitId"`
	Comment  string `json:"comment"`
}

// AuthorModel ...
type AuthorModel struct {
	DisplayName string `json:"displayName"`
}

// RefUpdatesModel ...
type RefUpdatesModel struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId"`
	NewObjectID string `json:"newObjectId"`
}

// PushResourceModel ...
type PushResourceModel struct {
	Commits    []CommitModel     `json:"commits"`
	RefUpdates []RefUpdatesModel `json:"refUpdates"`
}

// PullRequestResourceModel ...
type PullRequestResourceModel struct {
	SourceReferenceName string      `json:"sourceRefName"`
	TargetReferenceName string      `json:"targetRefName"`
	MergeStatus         string      `json:"mergeStatus"`
	LastSourceCommit    CommitModel `json:"lastMergeSourceCommit"`
	CreatedBy           AuthorModel `json:"createdBy"`
	Status              string      `json:"status"`
	PullRequestID       int         `json:"pullRequestId"`
}

// EventMessage ...
type EventMessage struct {
	Text string `json:"text"`
}

// EventModel ...
type EventModel struct {
	SubscriptionID string `json:"subscriptionId"`
	EventType      string `json:"eventType"`
	PublisherID    string `json:"publisherId"`
}

// PushEventModel ...
type PushEventModel struct {
	SubscriptionID  string            `json:"subscriptionId"`
	EventType       string            `json:"eventType"`
	PublisherID     string            `json:"publisherId"`
	Resource        PushResourceModel `json:"resource"`
	ResourceVersion string            `json:"resourceVersion"`
	DetailedMessage EventMessage      `json:"detailedMessage"`
	Message         EventMessage      `json:"message"`
}

// PullRequestEventModel ...
type PullRequestEventModel struct {
	SubscriptionID  string                   `json:"subscriptionId"`
	EventType       string                   `json:"eventType"`
	PublisherID     string                   `json:"publisherId"`
	Resource        PullRequestResourceModel `json:"resource"`
	ResourceVersion string                   `json:"resourceVersion"`
	DetailedMessage EventMessage             `json:"detailedMessage"`
	Message         EventMessage             `json:"message"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

// detectContentType ...
func detectContentType(header http.Header) (string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", errors.New("No Content-Type Header found")
	}

	return contentType, nil
}

// transformPushEvent ...
func transformPushEvent(pushEvent PushEventModel) hookCommon.TransformResultModel {
	if pushEvent.ResourceVersion != "1.0" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported resource version"),
		}
	}

	if len(pushEvent.Resource.RefUpdates) != 1 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Can't detect branch information (resource.refUpdates is empty), can't start a build"),
		}
	}

	headRefUpdate := pushEvent.Resource.RefUpdates[0]
	pushRef := headRefUpdate.Name
	if strings.HasPrefix(pushRef, "refs/heads/") {
		// code push
		branch := strings.TrimPrefix(pushRef, "refs/heads/")

		if len(pushEvent.Resource.Commits) < 1 {
			var commitMessage string
			commitHash := headRefUpdate.NewObjectID
			if commitHash == emptyCommitHash {
				// no commits and the (new) commit hash is empty -> this is a delete event,
				// the branch was deleted
				return hookCommon.TransformResultModel{
					Error:      fmt.Errorf("Branch delete event - does not require a build"),
					ShouldSkip: true,
				}
			}
			if headRefUpdate.OldObjectID == emptyCommitHash {
				commitMessage = "Branch created"
				// (new) commit hash was not empty, but old one is -> this is a create event,
				// without any commits pushed, just the branch created
				return hookCommon.TransformResultModel{
					TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
						{
							BuildParams: bitriseapi.BuildParamsModel{
								Branch:         branch,
								CommitHash:     commitHash,
								CommitMessage:  commitMessage,
								CommitMessages: []string{commitMessage},
							},
						},
					},
				}
			}

			if commitHash != "" && headRefUpdate.OldObjectID != "" {
				// Both old and new commit hash defined in the head ref update,
				// but no "commits" info - this happens right now when you merge
				// a Pull Request on visualstudio.com
				// It will generate a commit and webhook, you can see the commit in
				// `git log`, but it does not include it in the hook event,
				// only the head ref change.
				// So, for now, we'll use the event's detailed message as the commit message.
				commitMessage = pushEvent.DetailedMessage.Text
				return hookCommon.TransformResultModel{
					TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
						{
							BuildParams: bitriseapi.BuildParamsModel{
								Branch:         branch,
								CommitHash:     commitHash,
								CommitMessage:  commitMessage,
								CommitMessages: []string{commitMessage},
							},
						},
					},
				}
			}

			// in every other case:
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("No 'commits' included in the webhook, can't start a build"),
			}
		}
		// Commits are in descending order, by commit date-time (first one is the latest)
		headCommit := pushEvent.Resource.Commits[0]

		var commitMessages []string
		for _, commit := range pushEvent.Resource.Commits {
			commitMessages = append(commitMessages, commit.Comment)
		}
		slices.Reverse(commitMessages)

		return hookCommon.TransformResultModel{
			TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Branch:         branch,
						CommitHash:     headCommit.CommitID,
						CommitMessage:  headCommit.Comment,
						CommitMessages: commitMessages,
					},
				},
			},
		}
	} else if strings.HasPrefix(pushRef, "refs/tags/") {
		// tag push
		tag := strings.TrimPrefix(pushRef, "refs/tags/")
		commitHash := headRefUpdate.NewObjectID
		if commitHash == emptyCommitHash {
			// deleted
			return hookCommon.TransformResultModel{
				Error:      fmt.Errorf("Tag delete event - does not require a build"),
				ShouldSkip: true,
			}
		}

		return hookCommon.TransformResultModel{
			TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Tag:        tag,
						CommitHash: commitHash,
					},
				},
			},
		}
	}

	return hookCommon.TransformResultModel{
		Error: fmt.Errorf("Unsupported refs/, can't start a build: %s", pushRef),
	}

}

// transformPullRequestEvent ...
func transformPullRequestEvent(pullRequestEvent PullRequestEventModel) hookCommon.TransformResultModel {
	if pullRequestEvent.ResourceVersion != "1.0" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported resource version"),
		}
	}

	pullRequest := pullRequestEvent.Resource
	if pullRequest.Status == "completed" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Pull request already completed"),
			ShouldSkip: true,
		}
	}

	if pullRequest.MergeStatus != "succeeded" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Pull request is not mergeable"),
			ShouldSkip: true,
		}
	}

	if pullRequest.SourceReferenceName == "" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Missing source reference name"),
		}
	}

	if !strings.HasPrefix(pullRequest.SourceReferenceName, "refs/heads/") {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Invalid source reference name"),
		}
	}

	if pullRequest.TargetReferenceName == "" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Missing target reference name"),
		}
	}

	if !strings.HasPrefix(pullRequest.TargetReferenceName, "refs/heads/") {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Invalid target reference name"),
		}
	}

	if pullRequest.LastSourceCommit == (CommitModel{}) || pullRequest.LastSourceCommit.CommitID == "" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Missing last source branch commit details"),
		}
	}

	var buildParams = bitriseapi.BuildParamsModel{
		CommitHash:        pullRequest.LastSourceCommit.CommitID,
		CommitMessage:     pullRequestEvent.Message.Text,
		Branch:            strings.TrimPrefix(pullRequest.SourceReferenceName, "refs/heads/"),
		BranchDest:        strings.TrimPrefix(pullRequest.TargetReferenceName, "refs/heads/"),
		PullRequestAuthor: pullRequest.CreatedBy.DisplayName,
	}

	if pullRequest.PullRequestID != 0 {
		buildParams.PullRequestID = &pullRequest.PullRequestID
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: buildParams,
			},
		},
	}
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, err := detectContentType(r.Header)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: err,
		}
	}
	matched, err := regexp.MatchString("application/json", contentType)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Issue with Header checking: %s", err),
		}
	}

	if matched != true {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read request body"),
		}
	}

	var event EventModel
	if err := json.Unmarshal(body, &event); err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
		}
	}

	if event.PublisherID != "tfs" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Not a Team Foundation Server notification, can't start a build"),
		}
	}

	if event.SubscriptionID == "00000000-0000-0000-0000-000000000000" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Initial (test) event detected, skipping"),
			ShouldSkip: true,
		}
	}

	if event.EventType == Push {
		var pushEvent PushEventModel
		if err := json.Unmarshal(body, &pushEvent); err != nil {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
			}
		}
		return transformPushEvent(pushEvent)
	} else if event.EventType == PullRequestCreate || event.EventType == PullRequestUpdate {
		var pullRequestEvent PullRequestEventModel
		if err := json.Unmarshal(body, &pullRequestEvent); err != nil {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
			}
		}
		return transformPullRequestEvent(pullRequestEvent)
	} else {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported event type"),
		}
	}

}
