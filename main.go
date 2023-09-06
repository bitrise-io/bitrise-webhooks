package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/bitrise-io/bitrise-webhooks/config"
	"github.com/bitrise-io/bitrise-webhooks/internal/pubsub"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func main() {
	tracer.Start(tracer.WithServiceName("webhooks"))
	defer tracer.Stop()
	var (
		portFlag          = flag.String("port", "", `Use port [$PORT]`)
		sendRequestToFlag = flag.String("send-request-to", "", `Send requests to this URL. If set, every request will be sent to this URL and not to bitrise.io. You can use this to debug/test, e.g. with http://requestb.in [$SEND_REQUEST_TO]`)
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
	setupRoutes(pubsubClient)

	log.Println("Starting - using port:", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to ListenAndServe: %s", err)
	}
}

func stringFlagOrEnv(flagValue *string, envKey string) string {
	if flagValue != nil && *flagValue != "" {
		return *flagValue
	}
	return os.Getenv(envKey)
}
