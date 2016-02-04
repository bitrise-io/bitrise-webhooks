package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/bitrise-io/bitrise-webhooks/providers"
	"github.com/bitrise-io/bitrise-webhooks/providers/bitbucketv2"
	"github.com/bitrise-io/bitrise-webhooks/providers/github"
)

// HookRespModel ...
type HookRespModel struct {
	Message string `json:"message"`
}

const (
	hookTypeIDGithub      = "github"
	hookTypeIDBitbucketV1 = "bitbucket-v1"
	hookTypeIDBitbucketV2 = "bitbucket-v2"
)

// func hookTypeCheckBitbucketV1(header http.Header) hookTypeModel {
// 	userAgents := header["User-Agent"]
// 	contentTypes := header["Content-Type"]
//
// 	if sliceutil.IsStringInSlice("Bitbucket.org", userAgents) &&
// 		sliceutil.IsStringInSlice("application/x-www-form-urlencoded", contentTypes) {
// 		return hookTypeModel{typeID: hookTypeIDBitbucketV1, isDontProcess: false}
// 	}
//
// 	return hookTypeModel{typeID: "", isDontProcess: false}
// }

func hookHandler(w http.ResponseWriter, r *http.Request) {
	supportedProviders := []providers.HookProvider{
		github.HookProvider{},
		bitbucketv2.HookProvider{},
	}

	var useProvider providers.HookProvider
	isProviderFound := false
	isCantTransform := false
	metrics.Trace("Determine hook type", func() {
		requestHeader := r.Header
		for _, aProvider := range supportedProviders {
			if hookCheckResult := aProvider.HookCheck(requestHeader); hookCheckResult.IsSupportedByProvider {
				// found the Provider
				useProvider = aProvider
				isProviderFound = true
				if hookCheckResult.IsCantTransform {
					// can't transform into a build
					isCantTransform = true
				}
				break
			}
		}
		// 	type = "bitbucket" if @body["canon_url"].eql?('https://bitbucket.org')
		log.Println("UNSUPPORTED webhook")
	})

	if !isProviderFound {
		respondWithBadRequestError(w, "Unsupported Webhook Type / Provider")
		return
	}

	if isCantTransform {
		resp := HookRespModel{
			Message: "Acknowledged, but skipping - not enough information to start a build, or unsupported event type",
		}
		respondWithSuccess(w, resp)
		return
	}

	resp := HookRespModel{
		Message: fmt.Sprintf("Processing: %#v", useProvider),
	}
	respondWithSuccess(w, resp)
}
