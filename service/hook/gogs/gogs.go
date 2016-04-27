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
	"github.com/bitrise-io/go-utils/httputil"
)

// --------------------------
// --- Webhook Data Model ---

const (
	pushEventID = "push"
)

// CommitModel ...
type CommitModel struct {
	CommitHash    string `json:"id"`
	CommitMessage string `json:"message"`
}

// CodePushEventModel ...
type CodePushEventModel struct {
	Secret      string        `json:"secret"`
	Ref         string        `json:"ref"`
	CheckoutSHA string        `json:"after"`
	Commits     []CommitModel `json:"commits"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentTypeAndEventID(header http.Header) (string, string, error) {
	contentType, err := httputil.GetSingleValueFromHeader("Content-Type", header)
	if err != nil {
		return "", "", fmt.Errorf("Issue with Content-Type Header: %s", err)
	}

	eventID, err := httputil.GetSingleValueFromHeader("X-Gogs-Event", header)
	if err != nil {
		return "", "", fmt.Errorf("Issue with X-Gogs-Event Header: %s", err)
	}

	return contentType, eventID, nil
}

func transformCodePushEvent(codePushEvent CodePushEventModel) hookCommon.TransformResultModel {
	if !strings.HasPrefix(codePushEvent.Ref, "refs/heads/") {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Ref (%s) is not a head ref", codePushEvent.Ref),
			ShouldSkip: true,
		}
	}
	branch := strings.TrimPrefix(codePushEvent.Ref, "refs/heads/")

	lastCommit := CommitModel{}
	isLastCommitFound := false
	for _, aCommit := range codePushEvent.Commits {
		if aCommit.CommitHash == codePushEvent.CheckoutSHA {
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

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    lastCommit.CommitHash,
					CommitMessage: lastCommit.CommitMessage,
					Branch:        branch,
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

	if contentType != "application/json" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if eventID != "push" {
		// Unsupported Event
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Unsupported Webhook event: %s", eventID),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	// code push
	var codePushEvent CodePushEventModel
	if contentType == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
			return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
		}
	}
	return transformCodePushEvent(codePushEvent)
}
