package bitbucketv2

import (
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/providers"
	"github.com/bitrise-io/go-utils/sliceutil"
)

// HookProvider ...
type HookProvider struct{}

// HookCheck ...
func (hp HookProvider) HookCheck(header http.Header) providers.HookCheckModel {
	userAgents := header["HTTP_USER_AGENT"]
	eventKeys := header["X-Event-Key"]

	if len(eventKeys) < 1 {
		// not a Bitbucket webhook
		return providers.HookCheckModel{IsSupportedByProvider: false, IsCantTransform: false}
	}

	isBitbucketAgent := false
	for _, aUserAgent := range userAgents {
		if strings.HasPrefix(aUserAgent, "Bitbucket-Webhooks/2") {
			isBitbucketAgent = true
		}
	}
	if !isBitbucketAgent {
		// not a Bitbucket webhook
		return providers.HookCheckModel{IsSupportedByProvider: false, IsCantTransform: false}
	}

	// check event type/key
	if sliceutil.IsStringInSlice("repo:push", eventKeys) {
		// We'll process this
		return providers.HookCheckModel{IsSupportedByProvider: true, IsCantTransform: false}
	}

	// Bitbucket webhook, but not supported event type - skip it
	return providers.HookCheckModel{IsSupportedByProvider: true, IsCantTransform: true}
}

// Transform ...
func (hp HookProvider) Transform(r *http.Request) providers.HookTransformResultModel {
	return providers.HookTransformResultModel{}
}
