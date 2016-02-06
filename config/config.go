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
