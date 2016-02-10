package hook

import (
	"fmt"
	"log"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/bitrise-webhooks/config"
	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/bitrise-io/bitrise-webhooks/service"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/github"
	"github.com/gorilla/mux"
)

func supportedProviders() map[string]hookCommon.Provider {
	return map[string]hookCommon.Provider{
		"github": github.HookProvider{},
		// "bitbucket-v2": bitbucketv2.HookProvider{},
	}
}

// RespModel ...
type RespModel struct {
	Message string `json:"message"`
}

// HTTPHandler ...
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceID := vars["service-id"]
	appSlug := vars["app-slug"]
	apiToken := vars["api-token"]

	if serviceID == "" {
		service.RespondWithBadRequestError(w, "No service-id defined")
		return
	}
	if appSlug == "" {
		service.RespondWithBadRequestError(w, "No App Slug parameter defined")
		return
	}
	if apiToken == "" {
		service.RespondWithBadRequestError(w, "No API Token parameter defined")
		return
	}

	hookProvider, isSupported := supportedProviders()[serviceID]
	if !isSupported {
		service.RespondWithBadRequestError(w, fmt.Sprintf("Unsupported Webhook Type / Provider: %s", serviceID))
		return
	}

	hookTransformResult := hookCommon.TransformResultModel{}
	metrics.Trace("Hook: Transform", func() {
		hookTransformResult = hookProvider.Transform(r)
	})

	if hookTransformResult.ShouldSkip {
		resp := RespModel{
			Message: fmt.Sprintf("Acknowledged, but skipping. Reason: %s", hookTransformResult.Error),
		}
		service.RespondWithSuccess(w, resp)
		return
	}
	if hookTransformResult.Error != nil {
		errMsg := fmt.Sprintf("Failed to transform the webhook: %s", hookTransformResult.Error)
		log.Printf(" (debug) %s", errMsg)
		service.RespondWithBadRequestError(w, errMsg)
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
				service.RespondWithBadRequestError(w, fmt.Sprintf("Failed to create Build Trigger URL: %s", err))
				return
			}
			url = u
		}

		isOnlyLog := !(config.SendRequestToURL != nil || config.GetServerEnvMode() == config.ServerEnvModeProd)

		responseFromServerBytes, err := bitriseapi.TriggerBuild(url, apiToken, hookTransformResult.TriggerAPIParams, isOnlyLog)
		if err != nil {
			service.RespondWithBadRequestError(w, fmt.Sprintf("Failed to Trigger the Build: %s", err))
			return
		}
		respondWithBytes = responseFromServerBytes
	})

	service.RespondWithSuccessJSONBytes(w, respondWithBytes)
}
