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
	Errors              []string
	TriggerAPIResponses []bitriseapi.TriggerAPIResponseModel
}

// ResponseTransformer ...
type ResponseTransformer interface {
	// TransformResponse ...
	TransformResponse(input TransformResponseInputModel) TransformResponseModel
	// TransformErrorMessageResponse ...
	TransformErrorMessagesResponse(errMsgs []string) TransformResponseModel
	// TransformSuccessMessageResponse ...
	TransformSuccessMessageResponse(msg string) TransformResponseModel
}
