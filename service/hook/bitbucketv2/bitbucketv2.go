package bitbucketv2

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/go-utils/httputil"
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

// CodePushEventModel ...
type CodePushEventModel struct {
	PushInfo PushInfoModel `json:"push"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentTypeAttemptNumberAndEventKey(header http.Header) (string, string, string, error) {
	contentType, err := httputil.GetSingleValueFromHeader("Content-Type", header)
	if err != nil {
		return "", "", "", fmt.Errorf("Issue with Content-Type Header: %s", err)
	}

	eventKey, err := httputil.GetSingleValueFromHeader("X-Event-Key", header)
	if err != nil {
		return "", "", "", fmt.Errorf("Issue with X-Event-Key Header: %s", err)
	}

	attemptNum, err := httputil.GetSingleValueFromHeader("X-Attempt-Number", header)
	if err != nil {
		return "", "", "", fmt.Errorf("Issue with X-Attempt-Number Header: %s", err)
	}

	return contentType, attemptNum, eventKey, nil
}

func transformCodePushEvent(codePushEvent CodePushEventModel) hookCommon.TransformResultModel {
	if len(codePushEvent.PushInfo.Changes) < 1 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("No 'changes' included in the webhook, can't start a build."),
		}
	}

	triggerAPIParams := []bitriseapi.TriggerAPIParamsModel{}
	errs := []string{}
	for _, aChnage := range codePushEvent.PushInfo.Changes {
		aNewItm := aChnage.ChangeNewItem
		if aNewItm.Type != "branch" {
			errs = append(errs, fmt.Sprintf("Not a type=branch change. Type was: %s", aNewItm.Type))
			continue
		}
		if aNewItm.Target.Type != "commit" {
			errs = append(errs, fmt.Sprintf("Target: Not a type=commit change. Type was: %s", aNewItm.Target.Type))
			continue
		}

		aTriggerAPIParams := bitriseapi.TriggerAPIParamsModel{
			BuildParams: bitriseapi.BuildParamsModel{
				CommitHash:    aNewItm.Target.CommitHash,
				CommitMessage: aNewItm.Target.CommitMessage,
				Branch:        aNewItm.Name,
			},
		}
		triggerAPIParams = append(triggerAPIParams, aTriggerAPIParams)
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

// Transform ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(r.Header)
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
	if eventKey != "repo:push" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("X-Event-Key is not supported: %s", eventKey),
		}
	}
	// Check: is this a re-try hook?
	if attemptNum != "1" {
		return hookCommon.TransformResultModel{
			ShouldSkip: true,
			Error:      fmt.Errorf("No retry is supported (X-Attempt-Number: %s)", attemptNum),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	var codePushEvent CodePushEventModel
	if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
		}
	}

	return transformCodePushEvent(codePushEvent)
}
