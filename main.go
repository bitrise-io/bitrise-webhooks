package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/bitrise-io/bitrise-webhooks/config"
	"github.com/bitrise-io/bitrise-webhooks/metrics"
)

func stringFlagOrEnv(flagValue *string, envKey string) string {
	if flagValue != nil && *flagValue != "" {
		return *flagValue
	}
	return os.Getenv(envKey)
}

func stringFlag(flagValue *string) string {
	if flagValue != nil && *flagValue != "" {
		return *flagValue
	}
	return ""
}

func main() {
	var (
		portFlag          = flag.String("port", "", `Use port [$PORT]`)
		sendRequestToFlag = flag.String("send-request-to", "", `Send requests to this URL. If set, every request will be sent to this URL and not to bitrise.io. You can use this to debug/test, e.g. with http://requestb.in [$SEND_REQUEST_TO]`)
		newRelicKeyFlag   = flag.String("newrelic", "", `NewRelic license key`)
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

	// Monitoring
	if config.GetServerEnvMode() == config.ServerEnvModeProd {
		newRelicKey := stringFlagOrEnv(newRelicKeyFlag, "NEW_RELIC_LICENSE_KEY")
		metrics.SetupNewRelic("BitriseWebhooksProcessor", newRelicKey)
	} else {
		log.Println(" (!) Skipping NewRelic setup - environment is not 'production'")
	}

	// Routing
	setupRoutes()

	log.Println("Starting - using port:", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to ListenAndServe: %s", err)
	}
}
