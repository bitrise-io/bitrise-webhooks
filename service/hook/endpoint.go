package hook

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"

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

func triggerBuild(triggerURL *url.URL, apiToken string, triggerAPIParams bitriseapi.TriggerAPIParamsModel) ([]byte, error) {
	isOnlyLog := !(config.SendRequestToURL != nil || config.GetServerEnvMode() == config.ServerEnvModeProd)

	responseFromServerBytes, err := bitriseapi.TriggerBuild(triggerURL, apiToken, triggerAPIParams, isOnlyLog)
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to Trigger the Build: %s", err)
	}
	return responseFromServerBytes, nil
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

	// Let's Trigger a Build!
	triggerURL := config.SendRequestToURL
	if triggerURL == nil {
		u, err := bitriseapi.BuildTriggerURL("https://www.bitrise.io", appSlug)
		if err != nil {
			log.Printf(" [!] Exception: hookHandler: failed to create Build Trigger URL: %s", err)
			service.RespondWithBadRequestError(w, fmt.Sprintf("Failed to create Build Trigger URL: %s", err))
			return
		}
		triggerURL = u
	}

	respondWithBytes := []byte{}
	respondWithErrors := []error{}
	metrics.Trace("Hook: Trigger Builds", func() {
		if len(hookTransformResult.TriggerAPIParams) == 0 {
			respondWithErrors = append(respondWithErrors, errors.New("After processing the webhook we failed to detect any event in it which could be turned into a build."))
			return
		} else if len(hookTransformResult.TriggerAPIParams) == 1 {
			respBytes, err := triggerBuild(triggerURL, apiToken, hookTransformResult.TriggerAPIParams[0])
			if err != nil {
				respondWithErrors = append(respondWithErrors, err)
				return
			}
			respondWithBytes = respBytes
		} else {
			for _, aBuildTriggerParam := range hookTransformResult.TriggerAPIParams {
				if _, err := triggerBuild(triggerURL, apiToken, aBuildTriggerParam); err != nil {
					respondWithErrors = append(respondWithErrors, err)
				}
			}
		}
	})

	if len(respondWithErrors) > 0 {
		errorMsg := "Multiple Errors during Triggering Builds: "
		for idx, anError := range respondWithErrors {
			if idx != 0 {
				errorMsg += " | "
			}
			errorMsg += anError.Error()
		}
		service.RespondWithBadRequestError(w, errorMsg)
		return
	}

	service.RespondWithSuccessJSONBytes(w, respondWithBytes)
}
