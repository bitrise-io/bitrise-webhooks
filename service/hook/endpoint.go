package hook

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/bitrise-io/api-utils/logging"
	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/bitrise-webhooks/config"
	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/bitrise-io/bitrise-webhooks/service"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/assembla"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/bitbucketserver"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/bitbucketv2"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/deveo"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/github"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/gitlab"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/gogs"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/passthrough"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/slack"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/visualstudioteamservices"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func supportedProviders() map[string]hookCommon.Provider {
	return map[string]hookCommon.Provider{
		"github":           github.HookProvider{},
		"bitbucket-v2":     bitbucketv2.HookProvider{},
		"bitbucket-server": bitbucketserver.HookProvider{},
		"slack":            slack.HookProvider{},
		"visualstudio":     visualstudioteamservices.HookProvider{},
		"gitlab":           gitlab.HookProvider{},
		"gogs":             gogs.HookProvider{},
		"deveo":            deveo.HookProvider{},
		"assembla":         assembla.HookProvider{},
		"passthrough":      passthrough.HookProvider{},
	}
}

// ----------------------------------
// --- Response handler functions ---

func respondWithErrorString(w http.ResponseWriter, provider *hookCommon.Provider, errStr string) {
	responseProvider := hookCommon.ResponseTransformer(hookCommon.DefaultResponseProvider{})
	if provider != nil {
		if respTransformer, ok := (*provider).(hookCommon.ResponseTransformer); ok {
			// provider can transform responses - let it do so
			responseProvider = respTransformer
		}
	}
	//
	respInfo := responseProvider.TransformErrorMessageResponse(errStr)
	httpStatusCode := 400 // default
	if respInfo.HTTPStatusCode != 0 {
		httpStatusCode = respInfo.HTTPStatusCode
	}
	service.RespondWith(w, httpStatusCode, respInfo.Data)
}

func respondWithSuccessMessage(w http.ResponseWriter, provider *hookCommon.Provider, msg string) {
	responseProvider := hookCommon.ResponseTransformer(hookCommon.DefaultResponseProvider{})
	if provider != nil {
		if respTransformer, ok := (*provider).(hookCommon.ResponseTransformer); ok {
			// provider can transform responses - let it do so
			responseProvider = respTransformer
		}
	}
	//
	respInfo := responseProvider.TransformSuccessMessageResponse(msg)
	httpStatusCode := 201 // default
	if respInfo.HTTPStatusCode != 0 {
		httpStatusCode = respInfo.HTTPStatusCode
	}
	service.RespondWith(w, httpStatusCode, respInfo.Data)
}

func respondWithResults(w http.ResponseWriter, provider *hookCommon.Provider, results hookCommon.TransformResponseInputModel) {
	responseProvider := hookCommon.ResponseTransformer(hookCommon.DefaultResponseProvider{})
	if provider != nil {
		if respTransformer, ok := (*provider).(hookCommon.ResponseTransformer); ok {
			// provider can transform responses - let it do so
			responseProvider = respTransformer
		}
	}
	//
	respInfo := responseProvider.TransformResponse(results)
	httpStatusCode := 201 // default
	if respInfo.HTTPStatusCode != 0 {
		httpStatusCode = respInfo.HTTPStatusCode
	}
	service.RespondWith(w, httpStatusCode, respInfo.Data)
}

// -------------------------
// --- Utility functions ---

func triggerBuild(triggerURL *url.URL, apiToken string, triggerAPIParams bitriseapi.TriggerAPIParamsModel) (bitriseapi.TriggerAPIResponseModel, bool, error) {
	logger := logging.WithContext(nil)
	defer func() {
		if err := recover(); err != nil {
			//panic happened
			formattedError := fmt.Errorf("%s", err)
			logger.Error("PANIC happened ", zap.Error(formattedError))
		}
	}()

	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Println("Failed to Sync logger")
		}
	}()
	logger.Info(" ===> trigger build", zap.String("triggerURL", triggerURL.String()))
	isOnlyLog := !(config.SendRequestToURL != nil || config.GetServerEnvMode() == config.ServerEnvModeProd)
	if isOnlyLog {
		logger.Debug(colorstring.Yellow(" (debug) isOnlyLog: true"))
	}

	if err := triggerAPIParams.Validate(); err != nil {
		logger.Error(" (!) Failed to trigger build: invalid API parameters", zap.Error(err))
		return bitriseapi.TriggerAPIResponseModel{}, false, errors.Wrap(err, "Failed to Trigger the Build: Invalid parameters")
	}

	responseModel, isSuccess, err := bitriseapi.TriggerBuild(triggerURL, apiToken, triggerAPIParams, isOnlyLog)
	if err != nil {
		logger.Error(" [!] Exception: Failed to trigger build", zap.Error(err))
		return bitriseapi.TriggerAPIResponseModel{}, false, errors.Wrap(err, "Failed to Trigger the Build")
	}

	logger.Info(" ===> trigger build - DONE", zap.Bool("success", isSuccess), zap.String("triggerURL", triggerURL.String()))
	log.Printf("      (debug) response: (%#v)", responseModel)
	return responseModel, isSuccess, nil
}

// ------------------------------
// --- Main HTTP Handler code ---

// HTTPHandler ...
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceID := vars["service-id"]
	appSlug := vars["app-slug"]
	apiToken := vars["api-token"]

	logger := logging.WithContext(r.Context())
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Println("Failed to Sync logger")
		}
	}()

	if serviceID == "" {
		respondWithErrorString(w, nil, "No service-id defined")
		return
	}
	hookProvider, isSupported := supportedProviders()[serviceID]
	if !isSupported {
		respondWithErrorString(w, nil, fmt.Sprintf("Unsupported Webhook Type / Provider: %s", serviceID))
		return
	}

	if appSlug == "" {
		respondWithErrorString(w, &hookProvider, "No App Slug parameter defined")
		return
	}
	if apiToken == "" {
		respondWithErrorString(w, &hookProvider, "No API Token parameter defined")
		return
	}

	hookTransformResult := hookCommon.TransformResultModel{}
	metrics.Trace("Hook: Transform", func() {
		hookTransformResult = hookProvider.TransformRequest(r)
	})

	if hookTransformResult.ShouldSkip {
		respondWithSuccessMessage(w, &hookProvider, fmt.Sprintf("Acknowledged, but skipping. Reason: %s", hookTransformResult.Error))
		return
	}
	if hookTransformResult.Error != nil {
		errMsg := fmt.Sprintf("Failed to transform the webhook: %s", hookTransformResult.Error)
		log.Printf(" (debug) %s", errMsg)
		respondWithErrorString(w, &hookProvider, errMsg)
		return
	}

	// Let's Trigger a build / some builds!
	triggerURL := config.SendRequestToURL
	if triggerURL == nil {
		u, err := bitriseapi.BuildTriggerURL("https://app.bitrise.io", appSlug)
		if err != nil {
			logger.Error(" [!] Exception: hookHandler: failed to create Build Trigger URL", zap.Error(err))
			respondWithErrorString(w, &hookProvider, fmt.Sprintf("Failed to create Build Trigger URL: %s", err))
			return
		}
		triggerURL = u
	}

	buildTriggerCount := len(hookTransformResult.TriggerAPIParams)
	if buildTriggerCount == 0 {
		respondWithErrorString(w, &hookProvider, "After processing the webhook we failed to detect any event in it which could be turned into a build.")
		return
	}

	respondWith := hookCommon.TransformResponseInputModel{
		Errors:                       []string{},
		SuccessTriggerResponses:      []bitriseapi.TriggerAPIResponseModel{},
		SkippedTriggerResponses:      []hookCommon.SkipAPIResponseModel{},
		FailedTriggerResponses:       []bitriseapi.TriggerAPIResponseModel{},
		DidNotWaitForTriggerResponse: false,
	}
	metrics.Trace("Hook: Trigger Builds", func() {
		for _, aBuildTriggerParam := range hookTransformResult.TriggerAPIParams {
			commitMessage := aBuildTriggerParam.BuildParams.CommitMessage

			if hookCommon.IsSkipBuildByCommitMessage(commitMessage) {
				respondWith.SkippedTriggerResponses = append(respondWith.SkippedTriggerResponses, hookCommon.SkipAPIResponseModel{
					Message:       "Build skipped because the commit message included a skip ci keyword ([skip ci] or [ci skip]).",
					CommitHash:    aBuildTriggerParam.BuildParams.CommitHash,
					CommitMessage: aBuildTriggerParam.BuildParams.CommitMessage,
					Branch:        aBuildTriggerParam.BuildParams.Branch,
				})
				continue
			}

			triggerBuildAndPrepareRespondWith := func() {
				if triggerResp, isSuccess, err := triggerBuild(triggerURL, apiToken, aBuildTriggerParam); err != nil {
					respondWith.Errors = append(respondWith.Errors, fmt.Sprintf("Failed to Trigger Build: %s", err))
				} else if isSuccess {
					respondWith.SuccessTriggerResponses = append(respondWith.SuccessTriggerResponses, triggerResp)
				} else {
					respondWith.FailedTriggerResponses = append(respondWith.FailedTriggerResponses, triggerResp)
				}
			}

			if hookTransformResult.DontWaitForTriggerResponse {
				// send it, but don't wait for response
				go triggerBuildAndPrepareRespondWith()
				respondWith.DidNotWaitForTriggerResponse = true
			} else {
				// send and wait
				triggerBuildAndPrepareRespondWith()
			}
		}
	})

	respondWithResults(w, &hookProvider, respondWith)
}
