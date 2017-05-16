package common

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
)

const (
	// ContentTypeApplicationJSON ...
	ContentTypeApplicationJSON string = "application/json"
	// ContentTypeApplicationXWWWFormURLEncoded ...
	ContentTypeApplicationXWWWFormURLEncoded string = "application/x-www-form-urlencoded"
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
	// DontWaitForTriggerResponse if true the Trigger API request will be sent,
	//  but the handler won't wait for the response from the Trigger API,
	//  it'll respond immediately after calling the Trigger API
	DontWaitForTriggerResponse bool
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
// --- Response transformers ---

// TransformResponseModel ...
type TransformResponseModel struct {
	// Data will be transformed into JSON, and returned as the response.
	Data interface{}
	// HTTPStatusCode if specified (!= 0) will be used as the respone's
	//  HTTP response status code.
	HTTPStatusCode int
}

// SkipAPIResponseModel ...
type SkipAPIResponseModel struct {
	Message       string `json:"message"`
	CommitHash    string `json:"commit_hash"`
	CommitMessage string `json:"commit_message"`
	Branch        string `json:"branch"`
}

// TransformResponseInputModel ...
type TransformResponseInputModel struct {
	// Errors include the errors if the build could not trigger
	Errors []string

	// DidNotWaitForTriggerResponse if true it means that the TriggerResponses were not populated,
	//  as the provider requested to skip waiting for the Trigger API call's response/result.
	DidNotWaitForTriggerResponse bool

	// SuccessTriggerResponses include the successful trigger call responses
	SuccessTriggerResponses []bitriseapi.TriggerAPIResponseModel
	// FailedTriggerResponses include the trigger calls which were performed,
	//  but the response had a non success HTTP status code
	FailedTriggerResponses []bitriseapi.TriggerAPIResponseModel
	// SkippedTriggerResponses include responses for the trigger calls
	//  that were skipped
	SkippedTriggerResponses []SkipAPIResponseModel
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
