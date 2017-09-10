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
	envKeyHeaders = `WEBHOOK_HEADERS`
	envKeyBody    = `WEBHOOK_BODY`
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

	bodyBytes := []byte{}
	if r.Body != nil {
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to get request body: %s", err)}
		}
		bodyBytes = b
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
