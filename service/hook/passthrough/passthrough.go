package passthrough

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

const (
	envKeyHeaders      = `BITRISE_WEBHOOK_PASSTHROUGH_HEADERS`
	maxHeaderSizeBytes = 10 * 1024
	envKeyBody         = `BITRISE_WEBHOOK_PASSTHROUGH_BODY`
	maxBodySizeBytes   = 10 * 1024
)

// HookProvider ...
type HookProvider struct{}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	headerAsJSON := []byte{}
	if r.Header != nil {
		b, err := json.Marshal(r.Header)
		if err != nil {
			return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to JSON serialize request headers: %s", err)}
		}
		headerAsJSON = b
	}
	if len(headerAsJSON) > maxHeaderSizeBytes {
		return hookCommon.TransformResultModel{Error: fmt.Errorf("Headers too large, larger than %d bytes", maxHeaderSizeBytes)}
	}

	bodyBytes := []byte{}
	if r.Body != nil {
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to get request body: %s", err)}
		}
		bodyBytes = b
	}
	if len(bodyBytes) > maxBodySizeBytes {
		return hookCommon.TransformResultModel{Error: fmt.Errorf("Body too large, larger than %d bytes", maxBodySizeBytes)}
	}

	environments := []bitriseapi.EnvironmentItem{
		bitriseapi.EnvironmentItem{Name: envKeyHeaders, Value: string(headerAsJSON), IsExpand: false},
		bitriseapi.EnvironmentItem{Name: envKeyBody, Value: string(bodyBytes), IsExpand: false},
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:       "master",
					Environments: environments,
				},
			},
		},
	}
}
