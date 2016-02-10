package common

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
)

// TransformResultModel ...
type TransformResultModel struct {
	// TriggerAPIParams is the transformed Bitrise Trigger API params
	TriggerAPIParams bitriseapi.TriggerAPIParamsModel
	// ShouldSkip if true then no build should be started for this webhook
	//  but we should respond with a succcess HTTP status code
	ShouldSkip bool
	// Error in transforming the hook. If ShouldSkip=true this is
	//  the reason why the hook should be skipped.
	Error error
}

// Provider ...
type Provider interface {
	// Transform should transform the hook into a bitriseapi.TriggerAPIParamsModel
	//  which can then be called.
	// It might still decide to skip the actual call - for more info
	//  check the docs of TransformResultModel
	Transform(r *http.Request) TransformResultModel
}
