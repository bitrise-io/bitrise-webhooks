package hook

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/bitrise-io/api-utils/logging"
	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/bitrise-webhooks/config"
	"github.com/bitrise-io/bitrise-webhooks/internal/pubsub"
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
)

// Client ...
type Client struct {
	PubsubClient *pubsub.Client
}

func supportedProviders(logger *zap.Logger) map[string]hookCommon.Provider {
	return map[string]hookCommon.Provider{
		github.ProviderID:                   github.NewDefaultHookProvider(),
		bitbucketv2.ProviderID:              bitbucketv2.NewDefaultHookProvider(),
		bitbucketserver.ProviderID:          bitbucketserver.HookProvider{},
		slack.ProviderID:                    slack.HookProvider{},
		visualstudioteamservices.ProviderID: visualstudioteamservices.HookProvider{},
		gitlab.ProviderID:                   gitlab.NewDefaultHookProvider(logger),
		gogs.ProviderID:                     gogs.HookProvider{},
		deveo.ProviderID:                    deveo.HookProvider{},
		assembla.ProviderID:                 assembla.HookProvider{},
		passthrough.ProviderID:              passthrough.HookProvider{},
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

	logger.Info(" ===> trigger build", zap.String("triggerURL", triggerURL.String()))
	isOnlyLog := !(config.SendRequestToURL != nil || config.GetServerEnvMode() == config.ServerEnvModeProd)
	if isOnlyLog {
		logger.Debug(" \\x1b[33;1m(debug) isOnlyLog: true\\x1b[0m")
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
	logger.Debug(fmt.Sprintf("      (debug) response: (%#v)", responseModel))
	return responseModel, isSuccess, nil
}

// ------------------------------
// --- Main HTTP Handler code ---

// HTTPHandler ...
func (c *Client) HTTPHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceID := vars["service-id"]
	appSlug := vars["app-slug"]
	apiToken := vars["api-token"]

	reqContext := r.Context()

	logger := logging.WithContext(reqContext)

	if serviceID == "" {
		respondWithErrorString(w, nil, "No service-id defined")
		return
	}
	hookProvider, isSupported := supportedProviders(logger)[serviceID]
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

	metricsProvider, isMetricsProvider := hookProvider.(hookCommon.MetricsProvider)
	if c.PubsubClient != nil && isMetricsProvider {
		var webhookMetricsList []hookCommon.Metrics
		var err error

		metrics.Trace("Hook: GatherMetrics", func() {
			// GatherMetrics reads the request body, so it needs to be rewinded
			var originalBody []byte
			if r.Body != nil {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					logger.Error(" [!] Exception: failed to read request body", zap.Error(err))
					return
				}

				originalBody = body
			}
			if originalBody != nil {
				r.Body = io.NopCloser(bytes.NewBuffer(originalBody))
			}

			webhookMetricsList, err = metricsProvider.GatherMetrics(r, appSlug)

			if originalBody != nil {
				r.Body = io.NopCloser(bytes.NewBuffer(originalBody))
			}
		})

		if err != nil {
			knownErrors := []string{
				"unknown X-Github-Event in message",
				"payload signature check failed",
			}
			isKnownError := false

			for _, knownError := range knownErrors {
				if strings.Contains(err.Error(), knownError) {
					logger.Warn("Failed to gather metrics from the webhook", zap.Error(err))
					isKnownError = true
					break
				}
			}

			if !isKnownError {
				logger.Error("Failed to gather metrics from the webhook", zap.Error(err))
			}
		}

		if len(webhookMetricsList) > 0 {
			for _, webhookMetrics := range webhookMetricsList {
				if webhookMetrics == nil {
					continue
				}

				if err := c.PubsubClient.PublishMetrics(reqContext, webhookMetrics); err != nil {
					logger.Error(" [!] Exception: PublishMetrics: failed to publish metrics results", zap.Error(err))
				}
			}
		}
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
		logger.Debug(fmt.Sprintf(" (debug) %s", errMsg))
		respondWithErrorString(w, &hookProvider, errMsg)
		return
	}

	// Let's Trigger a build / some builds!
	triggerURL := config.SendRequestToURL
	if triggerURL == nil {
		u, err := bitriseapi.BuildTriggerURL("http://web-monolith.web-monolith:3000", appSlug)
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

	if hookTransformResult.SkippedByPrDescription {
		logger.Warn(fmt.Sprintf("[skipped by pr description] app: %s, service: %s", appSlug, serviceID))
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
				if aBuildTriggerParam.TriggeredBy == "" {
					aBuildTriggerParam.TriggeredBy = hookCommon.DefaultTriggeredBy
				}
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
