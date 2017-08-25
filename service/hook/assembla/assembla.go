package assembla

//
// Docs: https://articles.assembla.com/assembla-basics/learn-more/post-information-to-external-systems-using-webhooks
//

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

// --------------------------
// --- Webhook Data Model ---

// AssemblaEventModel ...
type AssemblaEventModel struct {
	Space	string	`json:"space"`
	Action	string	`json:"action"`
	Object	string	`json:"object"`
}

// MessageEventModel ...
type MessageEventModel struct {
	Title   string	`json:"title"`
	Body    string	`json:"body"`
	Author	string	`json:"author"`
}

// GitEventModel ...
type GitEventModel struct {
	RepositorySuffix	string      `json:"repository_suffix"`
	RepositoryURL		string      `json:"repository_url"`
	Branch				string      `json:"branch"`
	CommitId			string      `json:"commit_id"`
}

// PushEventModel ...
type PushEventModel struct {
	AssemblaEventModel	AssemblaEventModel	`json:"assembla"`
	MessageEventModel	MessageEventModel	`json:"message"`
	GitEventModel		GitEventModel		`json:"git"`
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

func detectAssemblaData(pushEvent PushEventModel) error {
	if (pushEvent.GitEventModel.CommitId == "---") ||
		(pushEvent.GitEventModel.Branch == "---") ||
		(pushEvent.GitEventModel.RepositoryURL == "---") ||
		(pushEvent.GitEventModel.RepositorySuffix == "---")	{
		return errors.New("Webhook is not correctly setup, make sure you post updates about 'Code commits' in Assembla")
	}

	return nil
}

func transformPushEvent(pushEvent PushEventModel) hookCommon.TransformResultModel {
	if pushEvent.AssemblaEventModel.Action != "committed" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Action was not 'committed', was: %s", pushEvent.AssemblaEventModel.Action),
		}
	}
	if pushEvent.MessageEventModel.Body == "" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Message body can't be empty"),
		}
	}
	if pushEvent.MessageEventModel.Author == "" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Message author can't be empty"),
		}
	}
	if pushEvent.GitEventModel.Branch == "" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Git branch can't be empty"),
		}
	}

	triggerAPIParams := []bitriseapi.TriggerAPIParamsModel{
		{
			BuildParams: bitriseapi.BuildParamsModel{
				CommitMessage: pushEvent.MessageEventModel.Body,
				Branch:        pushEvent.GitEventModel.Branch,
			},
			TriggeredBy: pushEvent.MessageEventModel.Author,
		},
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

	var pushEvent PushEventModel
	if err := json.NewDecoder(r.Body).Decode(&pushEvent); err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to parse request body as JSON: %s", err),
		}
	}

	return transformPushEvent(pushEvent)
}
