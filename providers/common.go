package providers

import "net/http"

// HookCheckModel ...
type HookCheckModel struct {
	// IsSupportedByProvider if true it means that the hook is related to
	//  the provider, and the provider should be used for processing it.
	IsSupportedByProvider bool
	// IsCantTransform if true it means that although the hook is related to
	//  the provider, the event it describes should be skipped, as it won't
	//  trigger a build. In this case the hook processing will return with a success response immediately,
	//  but it won't start a build.
	// An example: the hook is actually a GitHub webhook,
	//  but it was triggered by a Ticket event, and not by a code push / pull request event,
	//  and it can't be converted to a build.
	IsCantTransform bool
}

// HookProvider ...
type HookProvider interface {
	// HookCheck should return whether this provider supports
	//  the processing of the hook request.
	// It can also declare that the Hook is related to the provider,
	//  but the event itself should not be processed - it won't start a build.
	// For more information check the HookCheckModel's description
	HookCheck(header http.Header) HookCheckModel
}
