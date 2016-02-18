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

func getInputTextFromFormRequest(r *http.Request) (string, error) {
	triggerWord := r.FormValue("trigger_word")
	if len(triggerWord) > 0 {
		text := r.FormValue("text")
		if len(text) == 0 {
			return "", errors.New("'trigger_word' parameter found, but 'text' parameter is missing or empty")
		}
		text = strings.TrimSpace(strings.TrimPrefix(text, triggerWord))
		return text, nil
	}

	command := r.FormValue("command")
	if len(command) <= 0 {
		return "", errors.New("Missing required parameter: either 'command' or 'trigger_word' should be specified")
	}
	text := r.FormValue("text")
	if len(text) == 0 {
		return "", errors.New("'command' parameter found, but 'text' parameter is missing or empty")
	}
	text = strings.TrimSpace(text)

	return text, nil
}

func collectParamsFromPipeSeparatedText(text string) map[string]string {
	collectedParams := map[string]string{}

	splits := strings.Split(text, "|")
	for _, aItm := range splits {
		cleanedUpItm := strings.TrimSpace(aItm)
		if cleanedUpItm == "" {
			// skip, empty item
			continue
		}
		itmSplits := strings.Split(cleanedUpItm, ":")
		if len(itmSplits) < 2 {
			// skip, no split separator found
			continue
		}
		key := strings.TrimSpace(itmSplits[0])
		value := strings.TrimSpace(strings.Join(itmSplits[1:], ":"))
		collectedParams[key] = value
	}

	return collectedParams
}

func chooseFirstNonEmptyString(strs ...string) string {
	for _, aStr := range strs {
		if aStr != "" {
			return aStr
		}
	}
	return ""
}

func transformOutgoingWebhookMessage(slackText string) hookCommon.TransformResultModel {
	cleanedUpText := strings.TrimSpace(slackText)

	collectedParams := collectParamsFromPipeSeparatedText(cleanedUpText)
	//
	branch := chooseFirstNonEmptyString(collectedParams["branch"], collectedParams["b"])
	workflowID := chooseFirstNonEmptyString(collectedParams["workflow"], collectedParams["w"])
	//
	message := chooseFirstNonEmptyString(collectedParams["message"], collectedParams["m"])
	commitHash := chooseFirstNonEmptyString(collectedParams["commit"], collectedParams["c"])
	tag := chooseFirstNonEmptyString(collectedParams["tag"], collectedParams["t"])

	if branch == "" && workflowID == "" {
		return hookCommon.TransformResultModel{
			Error: errors.New("Missing 'branch' and 'workflow' parameters - at least one of these is required"),
		}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        branch,
					CommitMessage: message,
					CommitHash:    commitHash,
					Tag:           tag,
					WorkflowID:    workflowID,
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

	slackText, err := getInputTextFromFormRequest(r)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to parse the request/message: %s", err),
		}
	}

	return transformOutgoingWebhookMessage(slackText)
}

// ----------------------------
// --- Response transformer ---

// OutgoingWebhookRespModel ...
type OutgoingWebhookRespModel struct {
	Text     string `json:"text"`
	Username string `json:"username,omitempty"`
}

// TransformResponse ...
func (hp HookProvider) TransformResponse(input hookCommon.TransformResponseInputModel) hookCommon.TransformResponseModel {
	responseText := "Results:"
	isError := false
	if len(input.Errors) > 0 {
		isError = true
		responseText += "\n*[!] Errors*:"
		for _, anErr := range input.Errors {
			responseText += fmt.Sprintf("\n* %s", anErr)
		}
	}
	if len(input.FailedTriggerResponses) > 0 {
		isError = true
		responseText += "\n*[!] Failed Triggers*:"
		for _, aFailedTrigResp := range input.FailedTriggerResponses {
			responseText += fmt.Sprintf("\n* %+v", aFailedTrigResp)
		}
	}
	if len(input.SuccessTriggerResponses) > 0 {
		if isError {
			responseText += "\n*Successful Triggers*:"
		} else {
			responseText += "\n*Success!* Details:"
		}
		for _, aSuccessTrigResp := range input.SuccessTriggerResponses {
			responseText += fmt.Sprintf("\n* %+v", aSuccessTrigResp)
		}
	}

	return hookCommon.TransformResponseModel{
		Data: OutgoingWebhookRespModel{
			Text: responseText,
		},
		HTTPStatusCode: 200,
	}
}

// TransformErrorMessageResponse ...
func (hp HookProvider) TransformErrorMessageResponse(errMsg string) hookCommon.TransformResponseModel {
	return hookCommon.TransformResponseModel{
		Data: OutgoingWebhookRespModel{
			Text: fmt.Sprintf("*[!] Error*: %s", errMsg),
		},
		HTTPStatusCode: 200,
	}
}

// TransformSuccessMessageResponse ...
func (hp HookProvider) TransformSuccessMessageResponse(msg string) hookCommon.TransformResponseModel {
	return hookCommon.TransformResponseModel{
		Data: OutgoingWebhookRespModel{
			Text: msg,
		},
		HTTPStatusCode: 200,
	}
}
