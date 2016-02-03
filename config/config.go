package config

import "os"

const (
	// ServerEnvModeDev ...
	ServerEnvModeDev = "development"
	// ServerEnvModeProd ...
	ServerEnvModeProd = "production"
)

var (
	serverEnvironmentMode = ServerEnvModeDev

	// SendRequestTo ...
	SendRequestTo = ""
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
