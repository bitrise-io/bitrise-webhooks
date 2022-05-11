package deveo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

const ProviderID = "deveo"

// --------------------------
// --- Webhook Data Model ---

// CommitModel ...
type CommitModel struct {
	Distinct      bool   `json:"distinct"`
	CommitHash    string `json:"id"`
	CommitMessage string `json:"message"`
}

// PushEventModel ...
type PushEventModel struct {
	Ref     string        `json:"ref"`
	Deleted bool          `json:"deleted"`
	Commits []CommitModel `json:"commits"`
}

// RepoInfoModel ...
type RepoInfoModel struct {
	SSHURL string `json:"ssh_url"`
}

// BranchInfoModel ...
type BranchInfoModel struct {
	Ref        string        `json:"ref"`
	CommitHash string        `json:"sha"`
	Repo       RepoInfoModel `json:"repo"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func transformPushEvent(pushEvent PushEventModel) hookCommon.TransformResultModel {
	if pushEvent.Deleted {
		return hookCommon.TransformResultModel{
			Error:      errors.New("This is a 'Deleted' event, no build can be started"),
			ShouldSkip: true,
		}
	}

	headCommit := pushEvent.Commits[0]

	if strings.HasPrefix(pushEvent.Ref, "refs/heads/") {
		// code push
		branch := strings.TrimPrefix(pushEvent.Ref, "refs/heads/")

		if len(headCommit.CommitHash) == 0 {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Missing commit hash"),
			}
		}

		return hookCommon.TransformResultModel{
			TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Branch:        branch,
						CommitHash:    headCommit.CommitHash,
						CommitMessage: headCommit.CommitMessage,
					},
				},
			},
		}
	} else if strings.HasPrefix(pushEvent.Ref, "refs/tags/") {
		// tag push
		tag := strings.TrimPrefix(pushEvent.Ref, "refs/tags/")

		if len(headCommit.CommitHash) == 0 {
			return hookCommon.TransformResultModel{
				Error: fmt.Errorf("Missing commit hash"),
			}
		}

		return hookCommon.TransformResultModel{
			TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Tag:           tag,
						CommitHash:    headCommit.CommitHash,
						CommitMessage: headCommit.CommitMessage,
					},
				},
			},
		}
	}

	return hookCommon.TransformResultModel{
		Error:      fmt.Errorf("Ref (%s) is not a head nor a tag ref", pushEvent.Ref),
		ShouldSkip: true,
	}
}

func detectContentTypeAndEventID(header http.Header) (string, string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", "", errors.New("No Content-Type Header found")
	}

	deveoEvent := header.Get("X-Deveo-Event")
	if deveoEvent == "" {
		return "", "", errors.New("No X-Deveo-Event Header found")
	}

	return contentType, deveoEvent, nil
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, deveoEvent, err := detectContentTypeAndEventID(r.Header)
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

	if deveoEvent != "push" {
		// Unsupported Deveo Event
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported Deveo Webhook event: %s", deveoEvent),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	if deveoEvent == "push" {
		// push (code & tag)
		var pushEvent PushEventModel
		if contentType == hookCommon.ContentTypeApplicationJSON {
			if err := json.NewDecoder(r.Body).Decode(&pushEvent); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
			}
		} else if contentType == hookCommon.ContentTypeApplicationXWWWFormURLEncoded {
			payloadValue := r.PostFormValue("payload")
			if payloadValue == "" {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: empty payload")}
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
	}

	return hookCommon.TransformResultModel{
		Error: fmt.Errorf("Unsupported Deveo event type: %s", deveoEvent),
	}
}

// returns the repository clone URL
func (branchInfoModel BranchInfoModel) getRepositoryURL() string {
	return branchInfoModel.Repo.SSHURL
}
