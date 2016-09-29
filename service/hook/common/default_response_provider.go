package common

import (
	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
)

// DefaultResponseProvider ...
type DefaultResponseProvider struct {
}

// ErrorsRespModel ...
type ErrorsRespModel struct {
	Errors []string `json:"errors"`
}

// SingleErrorRespModel ...
type SingleErrorRespModel struct {
	Error string `json:"error"`
}

// SuccessRespModel ...
type SuccessRespModel struct {
	Message string `json:"message"`
}

// TransformResponseModel ...
type TransformResponseModel struct {
	Errors                  []string                             `json:"errors,omitempty"`
	SuccessTriggerResponses []bitriseapi.TriggerAPIResponseModel `json:"success_responses"`
	FailedTriggerResponses  []bitriseapi.TriggerAPIResponseModel `json:"failed_responses,omitempty"`
	SkippedTriggerResponses []SkipAPIResponseModel               `json:"skipped_responses,omitempty"`
}

// TransformResponse ...
func (hp DefaultResponseProvider) TransformResponse(input hookCommon.TransformResponseInputModel) hookCommon.TransformResponseModel {
	httpStatusCode := 201

	if len(input.SuccessTriggerResponses) == 0 && len(input.SkippedTriggerResponses) > 0 {
		httpStatusCode = 200
	}

	if len(input.Errors) > 0 || len(input.FailedTriggerResponses) > 0 {
		httpStatusCode = 400
	}

	return hookCommon.TransformResponseModel{
		Data: TransformResponseModel{
			Errors:                  input.Errors,
			SuccessTriggerResponses: input.SuccessTriggerResponses,
			FailedTriggerResponses:  input.FailedTriggerResponses,
			SkippedTriggerResponses: input.SkippedTriggerResponses,
		},
		HTTPStatusCode: httpStatusCode,
	}
}

// TransformErrorMessageResponse ...
func (hp DefaultResponseProvider) TransformErrorMessageResponse(errMsg string) hookCommon.TransformResponseModel {
	return hookCommon.TransformResponseModel{
		Data:           SingleErrorRespModel{Error: errMsg},
		HTTPStatusCode: 400,
	}
}

// TransformSuccessMessageResponse ...
func (hp DefaultResponseProvider) TransformSuccessMessageResponse(msg string) hookCommon.TransformResponseModel {
	return hookCommon.TransformResponseModel{
		Data:           SuccessRespModel{Message: msg},
		HTTPStatusCode: 200,
	}
}
