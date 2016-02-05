package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/bitrise-webhooks/config"
	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/bitrise-io/bitrise-webhooks/providers"
	"github.com/bitrise-io/bitrise-webhooks/providers/bitbucketv2"
	"github.com/bitrise-io/bitrise-webhooks/providers/github"
	"github.com/gorilla/mux"
)

// HookMessageRespModel ...
type HookMessageRespModel struct {
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

func selectProvider(header http.Header) (useProvider *providers.HookProvider, isCantTransform bool) {
	supportedProviders := []providers.HookProvider{
		github.HookProvider{},
		bitbucketv2.HookProvider{},
	}

	for _, aProvider := range supportedProviders {
		if hookCheckResult := aProvider.HookCheck(header); hookCheckResult.IsSupportedByProvider {
			// found the Provider
			useProvider = &aProvider
			isCantTransform = hookCheckResult.IsCantTransform
			return
		}
	}

	return
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appSlug := vars["app-slug"]
	apiToken := vars["api-token"]

	if appSlug == "" {
		respondWithBadRequestError(w, "No App Slug parameter defined")
		return
	}
	if apiToken == "" {
		respondWithBadRequestError(w, "No API Token parameter defined")
		return
	}

	var useProvider *providers.HookProvider
	isCantTransform := false
	metrics.Trace("Hook: determine type", func() {
		useProvider, isCantTransform = selectProvider(r.Header)
	})

	if useProvider == nil {
		respondWithBadRequestError(w, "Unsupported Webhook Type / Provider")
		return
	}

	if isCantTransform {
		resp := HookMessageRespModel{
			Message: "Acknowledged, but skipping - not enough information to start a build, or unsupported event type",
		}
		respondWithSuccess(w, resp)
		return
	}

	hookTransformResult := providers.HookTransformResultModel{}
	metrics.Trace("Hook: Transform", func() {
		hookTransformResult = (*useProvider).Transform(r)
	})

	if hookTransformResult.ShouldSkip {
		resp := HookMessageRespModel{
			Message: fmt.Sprintf("Acknowledged, but skipping, because: %s", hookTransformResult.Error),
		}
		respondWithSuccess(w, resp)
		return
	}
	if hookTransformResult.Error != nil {
		errMsg := fmt.Sprintf("Failed to transform the webhook: %s", hookTransformResult.Error)
		log.Printf(" (debug) %s", errMsg)
		respondWithBadRequestError(w, errMsg)
		return
	}

	// do call
	respondWithBytes := []byte{}
	metrics.Trace("Hook: Trigger Build", func() {
		url := config.SendRequestToURL
		if url == nil {
			u, err := bitriseapi.BuildTriggerURL("https://www.bitrise.io", appSlug)
			if err != nil {
				log.Printf(" [!] Exception: hookHandler: failed to create Build Trigger URL: %s", err)
				respondWithBadRequestError(w, fmt.Sprintf("Failed to create Build Trigger URL: %s", err))
				return
			}
			url = u
		}

		isOnlyLog := !(config.SendRequestToURL != nil || config.GetServerEnvMode() == config.ServerEnvModeProd)

		responseFromServerBytes, err := bitriseapi.TriggerBuild(url, apiToken, hookTransformResult.TriggerAPIParams, isOnlyLog)
		if err != nil {
			respondWithBadRequestError(w, fmt.Sprintf("Failed to Trigger the Build: %s", err))
			return
		}
		respondWithBytes = responseFromServerBytes
	})

	respondWithSuccessJSONBytes(w, respondWithBytes)
}
