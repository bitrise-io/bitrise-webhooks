package root

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/config"
	"github.com/bitrise-io/bitrise-webhooks/service"
	"github.com/bitrise-io/bitrise-webhooks/version"
)

// RespModel ...
type RespModel struct {
	Message         string `json:"message"`
	Version         string `json:"version"`
	Time            string `json:"time"`
	EnvironmentMode string `json:"environment_mode"`
}

// HTTPHandler ...
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	resp := RespModel{
		Message:         "Welcome to bitrise-webhooks! You can find more information and setup guides at: https://github.com/bitrise-io/bitrise-webhooks",
		Version:         version.VERSION,
		Time:            fmt.Sprintf("%s", time.Now()),
		EnvironmentMode: config.GetServerEnvMode(),
	}

	service.RespondWithSuccessOK(w, resp)
}
