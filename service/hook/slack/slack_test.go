package slack

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/stretchr/testify/require"
)

func Test_detectContentType(t *testing.T) {
	t.Log("Proper Content-Type")
	{
		header := http.Header{
			"Content-Type": {"application/x-www-form-urlencoded"},
		}
		contentType, err := detectContentType(header)
		require.NoError(t, err)
		require.Equal(t, "application/x-www-form-urlencoded", contentType)
	}
	t.Log("Missing Content-Type")
	{
		header := http.Header{}
		contentType, err := detectContentType(header)
		require.EqualError(t, err, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
		require.Equal(t, "", contentType)
	}
}

func Test_createMessageModelFromFormRequest(t *testing.T) {
	t.Log("Proper Form content")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("trigger_word", "the trigger word")
		form.Add("text", "the text")
		request.PostForm = form

		messageModel, err := createMessageModelFromFormRequest(&request)
		require.NoError(t, err)
		require.Equal(t, MessageModel{
			TriggerText: "the trigger word",
			Text:        "the text",
		}, messageModel)
	}

	t.Log("Missing trigger_word")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("text", "the text")
		request.PostForm = form

		messageModel, err := createMessageModelFromFormRequest(&request)
		require.EqualError(t, err, "Missing required parameter: 'trigger_word'")
		require.Equal(t, MessageModel{}, messageModel)
	}
	t.Log("Missing text")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("trigger_word", "the trigger word")
		request.PostForm = form

		messageModel, err := createMessageModelFromFormRequest(&request)
		require.EqualError(t, err, "Missing required parameter: 'text'")
		require.Equal(t, MessageModel{}, messageModel)
	}
}

func Test_transformOutgoingWebhookMessage(t *testing.T) {
	t.Log("Should be OK")
	{
		webhookMsg := MessageModel{
			TriggerText: "bitrise:",
			Text:        "bitrise: branch=master",
		}

		hookTransformResult := transformOutgoingWebhookMessage(webhookMsg)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Should be OK")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/x-www-form-urlencoded"},
			},
		}
		form := url.Values{}
		form.Add("trigger_word", "bitrise:")
		form.Add("text", "bitrise: branch=master")
		request.PostForm = form

		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Unsupported Event Type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: application/json")
	}

	t.Log("Missing 'text' from request data")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type": {"application/x-www-form-urlencoded"},
			},
		}
		form := url.Values{}
		form.Add("trigger_word", "the trigger word")
		request.PostForm = form

		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to parse the request/message: Missing required parameter: 'text'")
	}
}
