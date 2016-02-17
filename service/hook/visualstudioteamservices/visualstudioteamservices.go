package visualstudioteamservices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/go-utils/httputil"
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
	Name string `json:"name"`
}

// ResourceModel ...
type ResourceModel struct {
	Commits    []CommitsModel    `json:"commits"`
	RefUpdates []RefUpdatesModel `json:"refUpdates"`
}

// CodePushEventModel ...
type CodePushEventModel struct {
	SubscriptionID string        `json:"subscriptionId"`
	EventType      string        `json:"eventType"`
	PublisherID    string        `json:"publisherId"`
	Resource       ResourceModel `json:"resource"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentType(header http.Header) (string, error) {
	contentType, err := httputil.GetSingleValueFromHeader("Content-Type", header)
	if err != nil {
		return "", fmt.Errorf("Issue with Content-Type Header: %s", err)
	}

	return contentType, nil
}

// transformCodePushEvent ...
func transformCodePushEvent(codePushEvent CodePushEventModel) hookCommon.TransformResultModel {
	if codePushEvent.PublisherID != "tfs" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Not a Team Foundation Server notification, can't start a build."),
		}
	}

	if codePushEvent.EventType != "git.push" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Not a code push event, can't start a build."),
		}
	}

	if codePushEvent.SubscriptionID == "00000000-0000-0000-0000-000000000000" {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Initial (test) event detected, skipping."),
			ShouldSkip: true,
		}
	}

	if len(codePushEvent.Resource.Commits) < 1 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("No 'changes' included in the webhook, can't start a build."),
		}
	}

	if len(codePushEvent.Resource.RefUpdates) != 1 {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Can't detect branch information, can't start a build."),
		}
	}

	if !strings.HasPrefix(codePushEvent.Resource.RefUpdates[0].Name, "refs/heads/") {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Badly formatted branch detected, can't start a build."),
		}
	}
	branch := strings.TrimPrefix(codePushEvent.Resource.RefUpdates[0].Name, "refs/heads/")

	triggerAPIParams := []bitriseapi.TriggerAPIParamsModel{}
	errs := []string{}

	for _, aCommit := range codePushEvent.Resource.Commits {
		aTriggerAPIParams := bitriseapi.TriggerAPIParamsModel{
			BuildParams: bitriseapi.BuildParamsModel{
				CommitHash:    aCommit.CommitID,
				CommitMessage: aCommit.Comment,
				Branch:        branch,
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

	var codePushEvent CodePushEventModel
	if err := json.NewDecoder(r.Body).Decode(&codePushEvent); err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
		}
	}

	return transformCodePushEvent(codePushEvent)
}
