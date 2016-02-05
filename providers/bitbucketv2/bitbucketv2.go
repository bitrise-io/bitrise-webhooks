package bitbucketv2

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/providers"
	"github.com/bitrise-io/go-utils/httputil"
)

// HookProvider ...
type HookProvider struct{}

// HookCheck ...
func (hp HookProvider) HookCheck(header http.Header) providers.HookCheckModel {
	if userAgent, err := httputil.GetSingleValueFromHeader("User-Agent", header); err != nil {
		return providers.HookCheckModel{IsSupportedByProvider: false}
	} else if !strings.HasPrefix(userAgent, "Bitbucket-Webhooks/2") {
		return providers.HookCheckModel{IsSupportedByProvider: false}
	}

	eventKey, err := httputil.GetSingleValueFromHeader("X-Event-Key", header)
	if err != nil {
		return providers.HookCheckModel{IsSupportedByProvider: false}
	}

	if eventKey == "repo:push" {
		// We'll process this
		return providers.HookCheckModel{IsSupportedByProvider: true}
	}

	// Bitbucket webhook, but not supported event type - skip it
	return providers.HookCheckModel{
		IsSupportedByProvider: true,
		CantTransformReason:   fmt.Errorf("Unsupported Bitbucket hook event type: %s", eventKey),
	}
}

// Transform ...
func (hp HookProvider) Transform(r *http.Request) providers.HookTransformResultModel {
	return providers.HookTransformResultModel{}
}
