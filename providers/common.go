package providers

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
)

// HookCheckModel ...
type HookCheckModel struct {
	// IsSupportedByProvider if true it means that the hook is related to
	//  the provider, and the provider should be used for processing it.
	IsSupportedByProvider bool
	// CantTransformReason if defined it means that although the hook is related to
	//  the provider, the event it describes should be skipped, as it won't
	//  trigger a build. In this case the hook processing will return with a success response immediately,
	//  but it won't start a build.
	// An example: the hook is actually a GitHub webhook,
	//  but it was triggered by a Ticket event, and not by a code push / pull request event,
	//  and it can't be converted to a build.
	CantTransformReason error
}

// HookTransformResultModel ...
type HookTransformResultModel struct {
	// TriggerAPIParams is the transformed Bitrise Trigger API params
	TriggerAPIParams bitriseapi.TriggerAPIParamsModel
	// ShouldSkip if true then no build should be started for this webhook
	//  but we should respond with a succcess HTTP status code
	ShouldSkip bool
	// Error in transforming the hook. If ShouldSkip=true this is
	//  the reason why the hook should be skipped.
	Error error
}

// HookProvider ...
type HookProvider interface {
	// HookCheck should return whether this provider supports
	//  the processing of the hook request.
	// It can also declare that the Hook is related to the provider,
	//  but the event itself should not be processed - it won't start a build.
	// For more information check the HookCheckModel's description
	HookCheck(header http.Header) HookCheckModel

	// Transform should transform the hook into a bitriseapi.TriggerAPIParamsModel
	//  which can then be called.
	// It might still decide to skip the actual call - for more info
	//  check the docs of HookTransformResultModel
	Transform(r *http.Request) HookTransformResultModel
}
