package config

import (
	"net/url"
	"os"
)

const (
	// ServerEnvModeDev ...
	ServerEnvModeDev = "development"
	// ServerEnvModeProd ...
	ServerEnvModeProd = "production"
)

var (
	serverEnvironmentMode = ServerEnvModeDev

	// SendRequestToURL ...
	SendRequestToURL *url.URL

	// BuildTriggerURL URL to trigger builds (Website)
	BuildTriggerURL *url.URL

	// LogOnlyMode when set to true, no requests are sent to trigger builds
	LogOnlyMode = false
)

// GetServerEnvMode ...
func GetServerEnvMode() string {
	return serverEnvironmentMode
}

// SetupServerEnvMode ...
func SetupServerEnvMode() {
	envMode := os.Getenv("RACK_ENV")
	if envMode != "" {
		serverEnvironmentMode = envMode
	}
}
