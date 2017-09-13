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
					Branch:        "master",
					CommitMessage: "",
					Environments: []bitriseapi.EnvironmentItem{
						bitriseapi.EnvironmentItem{Name: "BITRISE_WEBHOOK_PASSTHROUGH_HEADERS", Value: "", IsExpand: false},
						bitriseapi.EnvironmentItem{Name: "BITRISE_WEBHOOK_PASSTHROUGH_BODY", Value: "", IsExpand: false},
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
					Branch:        "master",
					CommitMessage: bodyContent,
					Environments: []bitriseapi.EnvironmentItem{
						bitriseapi.EnvironmentItem{Name: "BITRISE_WEBHOOK_PASSTHROUGH_HEADERS", Value: `{"Content-Type":["application/json"],"Some-Custom-Header-List":["first-value","second-value"]}`, IsExpand: false},
						bitriseapi.EnvironmentItem{Name: "BITRISE_WEBHOOK_PASSTHROUGH_BODY", Value: bodyContent, IsExpand: false},
					},
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("If body is longer than max commit message length commit message will be a trimmed version of body")
	{
		for _, tc := range []struct {
			bodyContent           string
			expectedCommitMessage string
		}{
			{bodyContent: "a short body content, no trim", expectedCommitMessage: "a short body content, no trim"},
			{bodyContent: strings.Repeat("a", 100), expectedCommitMessage: strings.Repeat("a", 100)},
			{bodyContent: strings.Repeat("a", 100+1), expectedCommitMessage: strings.Repeat("a", 97) + "..."},
		} {
			request := http.Request{
				Header: http.Header{
					"Content-Type":            {"application/json"},
					"Some-Custom-Header-List": {"first-value", "second-value"},
				},
				Body: ioutil.NopCloser(strings.NewReader(tc.bodyContent)),
			}
			hookTransformResult := provider.TransformRequest(&request)
			require.NoError(t, hookTransformResult.Error)
			require.False(t, hookTransformResult.ShouldSkip)
			require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
				{
					BuildParams: bitriseapi.BuildParamsModel{
						Branch:        "master",
						CommitMessage: tc.expectedCommitMessage,
						Environments: []bitriseapi.EnvironmentItem{
							bitriseapi.EnvironmentItem{Name: "BITRISE_WEBHOOK_PASSTHROUGH_HEADERS", Value: `{"Content-Type":["application/json"],"Some-Custom-Header-List":["first-value","second-value"]}`, IsExpand: false},
							bitriseapi.EnvironmentItem{Name: "BITRISE_WEBHOOK_PASSTHROUGH_BODY", Value: tc.bodyContent, IsExpand: false},
						},
					},
				},
			}, hookTransformResult.TriggerAPIParams)
			require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
		}
	}

	t.Log("Body too large")
	{
		request := http.Request{
			Body: ioutil.NopCloser(strings.NewReader(strings.Repeat("a", 10*1024+1))),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Body too large, larger than 10240 bytes")
	}

	t.Log("Headers too large")
	{
		request := http.Request{
			Header: http.Header{
				"Some-Custom-Header-List": {"first-value", "second-value", strings.Repeat("a", 10*1024+1)},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Headers too large, larger than 10240 bytes")
	}
}
