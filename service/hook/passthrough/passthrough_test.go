package passthrough

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/stretchr/testify/require"
)

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Empty headers & body")
	{
		request := http.Request{}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "master",
					Environments: []bitriseapi.EnvironmentItem{
						bitriseapi.EnvironmentItem{Name: "WEBHOOK_HEADERS", Value: "", IsExpand: false},
						bitriseapi.EnvironmentItem{Name: "WEBHOOK_BODY", Value: "", IsExpand: false},
					},
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Request with headers & body")
	{
		bodyContent := `A simple,

multi line body
content.`
		request := http.Request{
			Header: http.Header{
				"Content-Type":            {"application/json"},
				"Some-Custom-Header-List": {"first-value", "second-value"},
			},
			Body: ioutil.NopCloser(strings.NewReader(bodyContent)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "master",
					Environments: []bitriseapi.EnvironmentItem{
						bitriseapi.EnvironmentItem{Name: "WEBHOOK_HEADERS", Value: `{"Content-Type":["application/json"],"Some-Custom-Header-List":["first-value","second-value"]}`, IsExpand: false},
						bitriseapi.EnvironmentItem{Name: "WEBHOOK_BODY", Value: bodyContent, IsExpand: false},
					},
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}
