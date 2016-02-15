package hook

import (
	"fmt"

	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
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

// TransformResponse ...
func (hp DefaultResponseProvider) TransformResponse(input hookCommon.TransformResponseInputModel) hookCommon.TransformResponseModel {
	if len(input.Errors) > 0 {
		return hookCommon.TransformResponseModel{
			Data:           ErrorsRespModel{Errors: input.Errors},
			HTTPStatusCode: 400,
		}
	}

	buildTriggerCount := len(input.TriggerAPIResponses)
	successMsg := ""
	if buildTriggerCount == 1 {
		successMsg = "Successfully triggered 1 build."
	} else {
		successMsg = fmt.Sprintf("Successfully triggered %d builds.", buildTriggerCount)
	}
	return hookCommon.TransformResponseModel{
		Data:           SuccessRespModel{Message: successMsg},
		HTTPStatusCode: 201,
	}
}

// TransformErrorMessagesResponse ...
func (hp DefaultResponseProvider) TransformErrorMessagesResponse(errMsgs []string) hookCommon.TransformResponseModel {
	if len(errMsgs) == 1 {
		return hookCommon.TransformResponseModel{
			Data:           SingleErrorRespModel{Error: errMsgs[0]},
			HTTPStatusCode: 400,
		}
	}

	return hookCommon.TransformResponseModel{
		Data:           ErrorsRespModel{Errors: errMsgs},
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
