package slack

import (
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

// MessageModel ...
type MessageModel struct {
	TriggerText string // trigger_word
	Text        string // text
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

func createMessageModelFromFormRequest(r *http.Request) (MessageModel, error) {
	msgModel := MessageModel{}
	msgModel.TriggerText = r.FormValue("trigger_word")
	if len(msgModel.TriggerText) == 0 {
		return MessageModel{}, errors.New("Missing required parameter: 'trigger_word'")
	}
	msgModel.Text = r.FormValue("text")
	if len(msgModel.Text) == 0 {
		return MessageModel{}, errors.New("Missing required parameter: 'text'")
	}
	return msgModel, nil
}

func transformOutgoingWebhookMessage(webhookMsg MessageModel) hookCommon.TransformResultModel {
	cleanedUpText := strings.TrimSpace(
		strings.TrimPrefix(webhookMsg.Text, webhookMsg.TriggerText))

	splits := strings.Split(cleanedUpText, "|")
	branch := ""
	for _, aItm := range splits {
		cleanedUpItm := strings.TrimSpace(aItm)
		if strings.HasPrefix(cleanedUpItm, "branch=") {
			branch = strings.TrimPrefix(cleanedUpItm, "branch=")
		}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: branch,
				},
			},
		},
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
	if contentType != "application/x-www-form-urlencoded" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	msgModel, err := createMessageModelFromFormRequest(r)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to parse the request/message: %s", err),
		}
	}

	return transformOutgoingWebhookMessage(msgModel)
}
