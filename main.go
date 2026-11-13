package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"

	_ "go.uber.org/automaxprocs"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/bitrise-io/bitrise-webhooks/config"
	"github.com/bitrise-io/bitrise-webhooks/internal/pubsub"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/entire"
)

func main() {
	err := tracer.Start(tracer.WithService("webhooks"))
	if err != nil {
		log.Fatalf("Unable to start tracing: %s", err)
	}
	defer tracer.Stop()
	var (
		portFlag            = flag.String("port", "", `Use port [$PORT]`)
		sendRequestToFlag   = flag.String("send-request-to", "", `Send requests to this URL. If set, every request will be sent to this URL and not to bitrise.io. You can use this to debug/test, e.g. with http://requestb.in [$SEND_REQUEST_TO]`)
		logOnlyModeFlag     = flag.Bool("log-only-mode", false, `Only print log messages without triggering builds [$LOG_ONLY_MODE]`)
		buildTriggerURLFlag = flag.String("build-trigger-url", "", "URL to send build trigger requests to [$BUILD_TRIGGER_URL]")
	)
	flag.Parse()

	port := stringFlagOrEnv(portFlag, "PORT")
	if port == "" {
		log.Fatal("Port must be set")
	}
	config.SetupServerEnvMode()

	requestToStr := stringFlagOrEnv(sendRequestToFlag, "SEND_REQUEST_TO")
	if requestToStr != "" {
		url, err := url.Parse(requestToStr)
		if err != nil {
			log.Fatalf("Failed to parse send-request-to (%s) as a URL, error: %s", requestToStr, err)
		}
		config.SendRequestToURL = url
		log.Printf(" (!) Send-Request-To specified, every request will be sent to: %s", config.SendRequestToURL)
	}

	logOnlyMode := boolFlagOrEnv(logOnlyModeFlag, "LOG_ONLY_MODE")

	buildTriggerURL := stringFlagOrEnv(buildTriggerURLFlag, "BUILD_TRIGGER_URL")
	if requestToStr == "" && buildTriggerURL == "" {
		log.Printf("No send-request-to or build-trigger-url specified, will only log requests")
		logOnlyMode = true
	} else if buildTriggerURL != "" {
		url, err := url.Parse(buildTriggerURL)
		if err != nil {
			log.Fatalf("Failed to parse build-trigger-url (%s) as a URL, error: %s", buildTriggerURL, err)
		}
		config.BuildTriggerURL = url
	}

	config.LogOnlyMode = logOnlyMode

	var (
		pubsubServiceAccountJSON = os.Getenv("METRICS_PUBSUB_SERVICE_ACCOUNT_JSON")
		pubsubTopicID            = os.Getenv("METRICS_PUBSUB_TOPIC_ID")
		pubsubProjectID          = os.Getenv("METRICS_PUBSUB_PROJECT_ID")
		pubsubClient             *pubsub.Client
	)
	if len(pubsubServiceAccountJSON) > 0 && len(pubsubTopicID) > 0 && len(pubsubProjectID) > 0 {
		var err error
		pubsubClient, err = pubsub.NewClient(pubsubProjectID, pubsubServiceAccountJSON, pubsubTopicID)
		if err != nil {
			log.Fatalf("Failed to init pubsub client, error: %s", err)
		}
	}

	// // NewRelic
	// if newRelicKey := stringFlagOrEnv(newRelicKeyFlag, "NEW_RELIC_LICENSE_KEY"); newRelicKey != "" && config.GetServerEnvMode() == config.ServerEnvModeProd {
	// 	metrics.SetupNewRelic("BitriseWebhooksProcessor", newRelicKey)
	// } else {
	// 	log.Println(" (!) Skipping NewRelic setup - environment is not 'production' or no NEW_RELIC_LICENSE_KEY provided")
	// }

	// Routing
	setupRoutes(pubsubClient, entireWebhookHandler())

	log.Println("Starting - using port:", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to ListenAndServe: %s", err)
	}
}

// entireWebhookHandler returns the handler of Entire's webhook deliveries, or nil when Entire is not
// configured: the route exists only where the plugin's registration points at this service.
func entireWebhookHandler() http.Handler {
	pluginSlug := os.Getenv("ENTIRE_PLUGIN_SLUG")
	deliveryJWKSURL := os.Getenv("ENTIRE_DELIVERY_JWKS_URL")
	if pluginSlug == "" || deliveryJWKSURL == "" {
		return nil
	}

	// GitProviderService is served by the Monolith, next to the build trigger endpoint.
	gitProviderServiceURL := config.BuildTriggerURL
	if rawURL := os.Getenv("GIT_PROVIDER_SERVICE_URL"); rawURL != "" {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			log.Fatalf("Failed to parse GIT_PROVIDER_SERVICE_URL (%s) as a URL, error: %s", rawURL, err)
		}
		gitProviderServiceURL = parsedURL
	}
	if gitProviderServiceURL == nil {
		log.Fatal("Entire webhooks need BUILD_TRIGGER_URL or GIT_PROVIDER_SERVICE_URL to reach the GitProviderService")
	}

	handler, err := entire.NewHandler(entire.Config{
		PluginSlug:       pluginSlug,
		DeliveryJWKSURL:  deliveryJWKSURL,
		BuildTriggerURL:  config.BuildTriggerURL,
		SendRequestToURL: config.SendRequestToURL,
		LogOnlyMode:      config.LogOnlyMode,
	}, entire.NewGitProviderClient(gitProviderServiceURL))
	if err != nil {
		log.Fatalf("Failed to set up the Entire webhook handler, error: %s", err)
	}
	httpHandler, err := handler.HTTPHandler()
	if err != nil {
		log.Fatalf("Failed to set up the Entire webhook handler, error: %s", err)
	}

	log.Printf("Entire webhooks are handled on /h/entire for the plugin: %s", pluginSlug)
	return httpHandler
}

func stringFlagOrEnv(flagValue *string, envKey string) string {
	if flagValue != nil && *flagValue != "" {
		return *flagValue
	}
	return os.Getenv(envKey)
}

func boolFlagOrEnv(flagValue *bool, envKey string) bool {
	if flagValue != nil {
		return *flagValue
	}
	return os.Getenv(envKey) == "true"
}
