package common

import (
	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
)

// DefaultResponseProvider ...
type DefaultResponseProvider struct {
}

// SingleErrorRespModel ...
type SingleErrorRespModel struct {
	Error string `json:"error"`
}

// SuccessRespModel ...
type SuccessRespModel struct {
	Message string `json:"message"`
}

// DefaultTransformResponseModel ...
type DefaultTransformResponseModel struct {
	Errors                       []string                             `json:"errors,omitempty"`
	DidNotWaitForTriggerResponse bool                                 `json:"did_not_wait_for_trigger_response,omitempty"`
	SuccessTriggerResponses      []bitriseapi.TriggerAPIResponseModel `json:"success_responses"`
	FailedTriggerResponses       []bitriseapi.TriggerAPIResponseModel `json:"failed_responses,omitempty"`
	SkippedTriggerResponses      []SkipAPIResponseModel               `json:"skipped_responses,omitempty"`
}

// TransformResponse ...
func (hp DefaultResponseProvider) TransformResponse(input TransformResponseInputModel) TransformResponseModel {
	httpStatusCode := 201

	if input.DidNotWaitForTriggerResponse {
		httpStatusCode = 200
	}

	if len(input.SuccessTriggerResponses) == 0 && len(input.SkippedTriggerResponses) > 0 {
		httpStatusCode = 200
	}

	if len(input.Errors) > 0 {
		httpStatusCode = 400
	}

	if len(input.FailedTriggerResponses) > 0 {
		if hp.knownError(input) {
			httpStatusCode = 200
		} else {
			httpStatusCode = 400
		}
	}

	return TransformResponseModel{
		Data: DefaultTransformResponseModel{
			Errors: input.Errors,
			DidNotWaitForTriggerResponse: input.DidNotWaitForTriggerResponse,
			SuccessTriggerResponses:      input.SuccessTriggerResponses,
			FailedTriggerResponses:       input.FailedTriggerResponses,
			SkippedTriggerResponses:      input.SkippedTriggerResponses,
		},
		HTTPStatusCode: httpStatusCode,
	}
}

// TransformErrorMessageResponse ...
func (hp DefaultResponseProvider) TransformErrorMessageResponse(errMsg string) TransformResponseModel {
	return TransformResponseModel{
		Data:           SingleErrorRespModel{Error: errMsg},
		HTTPStatusCode: 400,
	}
}

// TransformSuccessMessageResponse ...
func (hp DefaultResponseProvider) TransformSuccessMessageResponse(msg string) TransformResponseModel {
	return TransformResponseModel{
		Data:           SuccessRespModel{Message: msg},
		HTTPStatusCode: 200,
	}
}

func (hp DefaultResponseProvider) knownError(input TransformResponseInputModel) bool {
	for _, response := range input.FailedTriggerResponses {
		if response.Status != "error" {
			return false
		}
	}
	return true
}
