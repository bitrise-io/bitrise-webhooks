package visualstudioteamservices

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

const (
	emptyCommitHash = "0000000000000000000000000000000000000000"

	// ProviderID ...
	ProviderID = "visualstudio"
)

// --------------------------
// --- Webhook Data Model ---

// CommitsModel ...
type CommitsModel struct {
	CommitID string `json:"commitId"`
	Comment  string `json:"comment"`
}

// RefUpdatesModel ...
type RefUpdatesModel struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId"`
	NewObjectID string `json:"newObjectId"`
}

// ResourceModel ...
type ResourceModel struct {
	Commits    []CommitsModel    `json:"commits"`
	RefUpdates []RefUpdatesModel `json:"refUpdates"`
}

// EventMessage ...
type EventMessage struct {
	Text string `json:"text"`
}

// PushEventModel ...
type PushEventModel struct {
	SubscriptionID  string        `json:"subscriptionId"`
	EventType       string        `json:"eventType"`
	PublisherID     string        `json:"publisherId"`
	Resource        ResourceModel `json:"resource"`
	DetailedMessage EventMessage  `json:"detailedMessage"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentType(header http.Header) (string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", errors.New("No Content-Type Header found")
	}

	return contentType, nil
}

// transformPushEvent ...
func transformPushEvent(pushEvent PushEventModel) hookCommon.TransformResultModel {
	if pushEvent.PublisherID != "tfs" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Not a Team Foundation Server notification, can't start a build"),
		}
	}

	if pushEvent.EventType != "git.push" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Not a push event, can't start a build"),
		}
	}

	if pushEvent.SubscriptionID == "00000000-0000-0000-0000-000000000000" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Initial (test) event detected, skipping"),
			ShouldSkip: true,
		}
	}

	// VSO sends separate events for separate event (branches, tags, etc.)

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
				// (new) commit hash was not empty, but old one is -> this is a create event,
				// without any commits pushed, just the branch created
				return hookCommon.TransformResultModel{
					TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
						{
							BuildParams: bitriseapi.BuildParamsModel{
								Branch:        branch,
								CommitHash:    commitHash,
								CommitMessage: "Branch created",
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
				return hookCommon.TransformResultModel{
					TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
						{
							BuildParams: bitriseapi.BuildParamsModel{
								Branch:        branch,
								CommitHash:    commitHash,
								CommitMessage: pushEvent.DetailedMessage.Text,
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

		return hookCommon.TransformResultModel{
			TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Branch:        branch,
						CommitHash:    headCommit.CommitID,
						CommitMessage: headCommit.Comment,
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

	var pushEvent PushEventModel
	if err := json.NewDecoder(r.Body).Decode(&pushEvent); err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
		}
	}

	return transformPushEvent(pushEvent)
}
