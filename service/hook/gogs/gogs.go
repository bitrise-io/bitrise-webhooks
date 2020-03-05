package gogs

// # Infos / notes:
//
// ## Webhook calls
//
// Official API docs: https://gogs.io/docs/features/webhook
//
// This module works very similarly to the Gitlab processor.
// Please look there for more discussion of its operation.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

// --------------------------
// --- Webhook Data Model ---

const (
	pushEventID   = "push"
	createEventID = "create"
)

// CommitModel ...
type CommitModel struct {
	CommitHash    string `json:"id"`
	CommitMessage string `json:"message"`
}

// PushEventModel ...
type PushEventModel struct {
	Secret      string        `json:"secret"`
	Ref         string        `json:"ref"`
	CheckoutSHA string        `json:"after"`
	Commits     []CommitModel `json:"commits"`
}

// CreateEventModel ...
type CreateEventModel struct {
	Secret  string `json:"secret"`
	Ref     string `json:"ref"`
	RefType string `json:"ref_type"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentTypeAndEventID(header http.Header) (string, string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", "", errors.New("No Content-Type Header found")
	}

	eventID := header.Get("X-Gogs-Event")
	if eventID == "" {
		return "", "", errors.New("No X-Gogs-Event Header found")
	}

	return contentType, eventID, nil
}

func transformPushEvent(pushEvent PushEventModel) hookCommon.TransformResultModel {
	if strings.HasPrefix(pushEvent.Ref, "refs/tags/") {
		return hookCommon.TransformResultModel{
			ShouldSkip: true,
		}
	}

	lastCommit := CommitModel{}
	isLastCommitFound := false
	for _, aCommit := range pushEvent.Commits {
		if aCommit.CommitHash == pushEvent.CheckoutSHA {
			isLastCommitFound = true
			lastCommit = aCommit
			break
		}
	}

	if !isLastCommitFound {
		return hookCommon.TransformResultModel{
			Error: errors.New("The commit specified by 'after' was not included in the 'commits' array - no match found"),
		}
	}

	if len(lastCommit.CommitHash) == 0 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Missing commit hash"),
		}
	}

	if !strings.HasPrefix(pushEvent.Ref, "refs/heads/") {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Ref (%s) is not a head ref", pushEvent.Ref),
		}
	}

	branch := strings.TrimPrefix(pushEvent.Ref, "refs/heads/")

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        branch,
					CommitHash:    lastCommit.CommitHash,
					CommitMessage: lastCommit.CommitMessage,
				},
			},
		},
	}
}

func transformCreateEvent(createEvent CreateEventModel) hookCommon.TransformResultModel {
	// Currently, we only support tag creation, not branches. Even if we wanted branch creation
	// to trigger a build we would have to wait for https://github.com/go-gitea/gitea/issues/286
	// to be in both Gogs and Gitea core, and well adopted/distributed.
	if createEvent.RefType != "tag" {
		return hookCommon.TransformResultModel{
			Error:      errors.New("Ignoring branch-create request"),
			ShouldSkip: true,
		}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag: createEvent.Ref,
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

	if contentType != hookCommon.ContentTypeApplicationJSON {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	if eventID == pushEventID {
		var pushEvent PushEventModel
		if err := json.NewDecoder(r.Body).Decode(&pushEvent); err != nil {
			return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
		}

		return transformPushEvent(pushEvent)

	} else if eventID == createEventID {
		var createEvent CreateEventModel
		if err := json.NewDecoder(r.Body).Decode(&createEvent); err != nil {
			return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
		}

		return transformCreateEvent(createEvent)
	}

	// Unsupported Event
	return hookCommon.TransformResultModel{
		Error: fmt.Errorf("Unsupported Webhook event: %s", eventID),
	}
}
