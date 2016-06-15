package common

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
)

// TransformResultModel ...
type TransformResultModel struct {
	// TriggerAPIParams is the transformed Bitrise Trigger API params
	TriggerAPIParams []bitriseapi.TriggerAPIParamsModel
	// ShouldSkip if true then no build should be started for this webhook
	//  but we should respond with a succcess HTTP status code
	ShouldSkip bool
	// Error in transforming the hook. If ShouldSkip=true this is
	//  the reason why the hook should be skipped.
	Error error
}

// Provider ...
type Provider interface {
	// TransformRequest should transform the hook into a bitriseapi.TriggerAPIParamsModel
	//  which can then be called.
	// It might still decide to skip the actual call - for more info
	//  check the docs of TransformResultModel
	TransformRequest(r *http.Request) TransformResultModel
}

// ---------------------------------------
// --- Optional, Response transformers ---

// TransformResponseModel ...
type TransformResponseModel struct {
	// Data will be transformed into JSON, and returned as the response.
	Data interface{}
	// HTTPStatusCode if specified (!= 0) will be used as the respone's
	//  HTTP response status code.
	HTTPStatusCode int
}

// TransformResponseInputModel ...
type TransformResponseInputModel struct {
	// Errors include the errors if the build could not trigger
	Errors                  []string
	SkippedTriggerResponses []bitriseapi.SkipAPIResponseModel

	// SuccessTriggerResponses include the successful trigger call responses
	SuccessTriggerResponses []bitriseapi.TriggerAPIResponseModel
	// FailedTriggerResponses include the trigger calls which were performed,
	//  but the response had a non success HTTP status code
	FailedTriggerResponses []bitriseapi.TriggerAPIResponseModel
}

// ResponseTransformer ...
type ResponseTransformer interface {
	// TransformResponse is called when the hook was successfully
	//  transformed into Bitrise API call(s); both if the actual
	//  Build Trigger was successful or failed.
	TransformResponse(input TransformResponseInputModel) TransformResponseModel
	// TransformErrorMessageResponse is called if an error prevents
	//  any Trigger call (missing parameter, un-transformable hook, ...)
	// If a Build Trigger can be called then `TransformResponse` will be
	//  called, even if the call fails.
	TransformErrorMessageResponse(errMsg string) TransformResponseModel
	// TransformSuccessMessageResponse is called if no Bitrise Trigger API
	//  call(s) can be initiated, but the response is still considered as
	//  success (e.g. if the hook should be skipped, with a success response,
	//   which is the case for GitHub's "ping" hook).
	TransformSuccessMessageResponse(msg string) TransformResponseModel
}
