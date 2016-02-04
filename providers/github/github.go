package github

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/providers"
)

// HookProvider ...
type HookProvider struct{}

// HookCheck ...
func (hp HookProvider) HookCheck(header http.Header) providers.HookCheckModel {
	ghEvents := header["HTTP_X_GITHUB_EVENT"]

	if len(ghEvents) < 1 {
		// not a GitHub webhook
		return providers.HookCheckModel{IsSupportedByProvider: false, IsCantTransform: false}
	}

	for _, aGHEvent := range ghEvents {
		if aGHEvent == "push" || aGHEvent == "pull_request" {
			// We'll process this
			return providers.HookCheckModel{IsSupportedByProvider: true, IsCantTransform: false}
		}
	}

	// GitHub webhook, but not supported event type - skip it
	return providers.HookCheckModel{IsSupportedByProvider: true, IsCantTransform: true}
}
