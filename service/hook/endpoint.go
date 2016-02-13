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
	"github.com/bitrise-io/bitrise-webhooks/service/hook/bitbucketv2"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/github"
	"github.com/gorilla/mux"
)

func supportedProviders() map[string]hookCommon.Provider {
	return map[string]hookCommon.Provider{
		"github":       github.HookProvider{},
		"bitbucket-v2": bitbucketv2.HookProvider{},
	}
}

// SuccessRespModel ...
type SuccessRespModel struct {
	Message string `json:"message"`
}

// ErrorsRespModel ...
type ErrorsRespModel struct {
	Errors []string `json:"errors"`
}

func respondWithSingleErrorStr(w http.ResponseWriter, errStr string) {
	service.RespondWithError(w, http.StatusBadRequest, errStr)
}

func respondWithErrors(w http.ResponseWriter, errs []error) {
	errStrs := []string{}
	for _, aErr := range errs {
		errStrs = append(errStrs, aErr.Error())
	}
	service.RespondWithErrorJSON(w, http.StatusBadRequest, ErrorsRespModel{Errors: errStrs})
}

func triggerBuild(triggerURL *url.URL, apiToken string, triggerAPIParams bitriseapi.TriggerAPIParamsModel) error {
	log.Printf(" ===> trigger build: %s", triggerURL)
	isOnlyLog := !(config.SendRequestToURL != nil || config.GetServerEnvMode() == config.ServerEnvModeProd)
	if isOnlyLog {
		log.Println(" (debug) isOnlyLog: true")
	}

	responseModel, err := bitriseapi.TriggerBuild(triggerURL, apiToken, triggerAPIParams, isOnlyLog)
	if err != nil {
		log.Printf(" [!] Exception: failed to trigger build: %s", err)
		return fmt.Errorf("Failed to Trigger the Build: %s", err)
	}
	log.Printf(" ===> trigger build - SUCCESS (%s)", triggerURL)
	log.Printf("      (debug) response: (%#v)", responseModel)
	return nil
}

// HTTPHandler ...
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceID := vars["service-id"]
	appSlug := vars["app-slug"]
	apiToken := vars["api-token"]

	if serviceID == "" {
		respondWithSingleErrorStr(w, "No service-id defined")
		return
	}
	if appSlug == "" {
		respondWithSingleErrorStr(w, "No App Slug parameter defined")
		return
	}
	if apiToken == "" {
		respondWithSingleErrorStr(w, "No API Token parameter defined")
		return
	}

	hookProvider, isSupported := supportedProviders()[serviceID]
	if !isSupported {
		respondWithSingleErrorStr(w, fmt.Sprintf("Unsupported Webhook Type / Provider: %s", serviceID))
		return
	}

	hookTransformResult := hookCommon.TransformResultModel{}
	metrics.Trace("Hook: Transform", func() {
		hookTransformResult = hookProvider.Transform(r)
	})

	if hookTransformResult.ShouldSkip {
		resp := SuccessRespModel{
			Message: fmt.Sprintf("Acknowledged, but skipping. Reason: %s", hookTransformResult.Error),
		}
		service.RespondWithSuccess(w, http.StatusCreated, resp)
		return
	}
	if hookTransformResult.Error != nil {
		errMsg := fmt.Sprintf("Failed to transform the webhook: %s", hookTransformResult.Error)
		log.Printf(" (debug) %s", errMsg)
		respondWithSingleErrorStr(w, errMsg)
		return
	}

	// Let's Trigger a Build!
	triggerURL := config.SendRequestToURL
	if triggerURL == nil {
		u, err := bitriseapi.BuildTriggerURL("https://www.bitrise.io", appSlug)
		if err != nil {
			log.Printf(" [!] Exception: hookHandler: failed to create Build Trigger URL: %s", err)
			respondWithSingleErrorStr(w, fmt.Sprintf("Failed to create Build Trigger URL: %s", err))
			return
		}
		triggerURL = u
	}

	respondWithErrs := []error{}
	buildTriggerCount := len(hookTransformResult.TriggerAPIParams)
	metrics.Trace("Hook: Trigger Builds", func() {
		if buildTriggerCount == 0 {
			respondWithErrs = append(respondWithErrs, errors.New("After processing the webhook we failed to detect any event in it which could be turned into a build."))
			return
		} else if buildTriggerCount == 1 {
			err := triggerBuild(triggerURL, apiToken, hookTransformResult.TriggerAPIParams[0])
			if err != nil {
				respondWithErrs = append(respondWithErrs, err)
				return
			}
		} else {
			for _, aBuildTriggerParam := range hookTransformResult.TriggerAPIParams {
				if err := triggerBuild(triggerURL, apiToken, aBuildTriggerParam); err != nil {
					respondWithErrs = append(respondWithErrs, err)
				}
			}
		}
	})

	if len(respondWithErrs) > 0 {
		respondWithErrors(w, respondWithErrs)
		return
	}

	successMsg := ""
	if buildTriggerCount == 1 {
		successMsg = "Successfully triggered 1 build."
	} else {
		successMsg = fmt.Sprintf("Successfully triggered %d builds.", buildTriggerCount)
	}
	service.RespondWithSuccess(w, http.StatusCreated, SuccessRespModel{Message: successMsg})
}
