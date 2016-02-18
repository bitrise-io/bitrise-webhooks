package slack

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
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

func Test_getInputTextFromFormRequest(t *testing.T) {
	t.Log("Proper Form content")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("trigger_word", "the trigger word")
		form.Add("text", "the trigger word        the text")
		request.PostForm = form

		text, err := getInputTextFromFormRequest(&request)
		require.NoError(t, err)
		require.Equal(t, "the text", text)
	}

	t.Log("Missing trigger_word")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("text", "the text")
		request.PostForm = form

		text, err := getInputTextFromFormRequest(&request)
		require.EqualError(t, err, "Missing required parameter: either 'command' or 'trigger_word' should be specified")
		require.Equal(t, "", text)
	}

	t.Log("Missing text - trigger_word")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("trigger_word", "the trigger word")
		request.PostForm = form

		text, err := getInputTextFromFormRequest(&request)
		require.EqualError(t, err, "'trigger_word' parameter found, but 'text' parameter is missing or empty")
		require.Equal(t, "", text)
	}

	t.Log("Missing text - command")
	{
		request := http.Request{}
		form := url.Values{}
		form.Add("command", "the-command")
		request.PostForm = form

		text, err := getInputTextFromFormRequest(&request)
		require.EqualError(t, err, "'command' parameter found, but 'text' parameter is missing or empty")
		require.Equal(t, "", text)
	}
}

func Test_chooseFirstNonEmptyString(t *testing.T) {
	require.Equal(t, "a", chooseFirstNonEmptyString("a", "b"))
	require.Equal(t, "b", chooseFirstNonEmptyString("", "b"))
	require.Equal(t, "b", chooseFirstNonEmptyString("", "b", ""))
	require.Equal(t, "b", chooseFirstNonEmptyString("", "b", "c"))
	require.Equal(t, "c", chooseFirstNonEmptyString("", "", "c"))
	require.Equal(t, "", chooseFirstNonEmptyString("", "", ""))
	require.Equal(t, "", chooseFirstNonEmptyString())
}

func Test_collectParamsFromPipeSeparatedText(t *testing.T) {
	t.Log("Single item - trimming")
	{
		texts := []string{
			"key: the value",
			"key : the value",
			"key :the value",
			"key :   the value   ",
			" key :   the value   ",
			"key: the value |",
		}
		for _, aText := range texts {
			collectedParams := collectParamsFromPipeSeparatedText(aText)
			require.Equal(t, map[string]string{"key": "the value"}, collectedParams)
		}
	}

	t.Log("Single item, includes :")
	{
		collectedParams := collectParamsFromPipeSeparatedText("key: the:value")
		require.Equal(t, map[string]string{"key": "the:value"}, collectedParams)
		collectedParams = collectParamsFromPipeSeparatedText("key: the :value")
		require.Equal(t, map[string]string{"key": "the :value"}, collectedParams)
		collectedParams = collectParamsFromPipeSeparatedText("key: the : value")
		require.Equal(t, map[string]string{"key": "the : value"}, collectedParams)
		collectedParams = collectParamsFromPipeSeparatedText("key: the  :  value")
		require.Equal(t, map[string]string{"key": "the  :  value"}, collectedParams)
		collectedParams = collectParamsFromPipeSeparatedText("key    : the : value")
		require.Equal(t, map[string]string{"key": "the : value"}, collectedParams)
	}

	t.Log("Multiple items")
	{
		collectedParams := collectParamsFromPipeSeparatedText("key1: value 1 |   key2 : value 2")
		require.Equal(t, map[string]string{
			"key1": "value 1",
			"key2": "value 2",
		},
			collectedParams)
	}

	t.Log("Multiple items - empty parts")
	{
		collectedParams := collectParamsFromPipeSeparatedText("|key1: value 1 |   key2 : value 2|")
		require.Equal(t, map[string]string{
			"key2": "value 2",
			"key1": "value 1",
		},
			collectedParams)
	}

	t.Log("Multiple items - formatting test")
	{
		collectedParams := collectParamsFromPipeSeparatedText("|key1: value 1 |   key2 : value 2 |key3:value 3")
		require.Equal(t, map[string]string{
			"key1": "value 1",
			"key3": "value 3",
			"key2": "value 2",
		},
			collectedParams)
	}
}

func Test_transformOutgoingWebhookMessage(t *testing.T) {
	t.Log("Should be OK")
	{
		slackText := "branch:master"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
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

	t.Log("Should be OK - space between param key&value")
	{
		slackText := " branch: master"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
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

	t.Log("Empty parameter component")
	{
		slackText := "branch: master | "

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
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

	t.Log("Message parameter")
	{
		slackText := "branch: master | message: this is the Commit Message param"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        "master",
					CommitMessage: "this is the Commit Message param",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Commit parameter")
	{
		slackText := "branch: master | commit: cmtHash123"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:     "master",
					CommitHash: "cmtHash123",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Tag parameter")
	{
		slackText := "tag: v1.0|branch : develop"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch: "develop",
					Tag:    "v1.0",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Workflow parameter")
	{
		slackText := "workflow: my-wf1"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					WorkflowID: "my-wf1",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("All parameters - long form")
	{
		slackText := "branch : develop | tag: v1.1|  message : this is:my message  | commit: cmtHash321 | workflow: primary-wf"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        "develop",
					Tag:           "v1.1",
					CommitHash:    "cmtHash321",
					CommitMessage: "this is:my message",
					WorkflowID:    "primary-wf",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("All parameters - short form")
	{
		slackText := "b: develop | t: v1.1|  m : this is:my message  | c: cmtHash321 | w: primary-wf"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Branch:        "develop",
					Tag:           "v1.1",
					CommitHash:    "cmtHash321",
					CommitMessage: "this is:my message",
					WorkflowID:    "primary-wf",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Missing branch parameter")
	{
		slackText := "message: only message"

		hookTransformResult := transformOutgoingWebhookMessage(slackText)
		require.EqualError(t, hookTransformResult.Error, "Missing 'branch' and 'workflow' parameters - at least one of these is required")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
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
		form.Add("text", "bitrise: branch:master")
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
		require.EqualError(t, hookTransformResult.Error, "Failed to parse the request/message: 'trigger_word' parameter found, but 'text' parameter is missing or empty")
	}
}

func Test_HookProvider_TransformResponse(t *testing.T) {
	provider := HookProvider{}

	t.Log("Single success")
	{
		baseRespModel := hookCommon.TransformResponseInputModel{
			SuccessTriggerResponses: []bitriseapi.TriggerAPIResponseModel{
				{
					Status:    "ok",
					Message:   "triggered build",
					Service:   "bitrise",
					AppSlug:   "app-slug",
					BuildSlug: "build-slug",
				},
			},
		}

		resp := provider.TransformResponse(baseRespModel)
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: `Results:
*Success!* Details:
* {Status:ok Message:triggered build Service:bitrise AppSlug:app-slug BuildSlug:build-slug}`,
			},
			HTTPStatusCode: 200,
		}, resp)
	}

	t.Log("Single failed trigger")
	{
		baseRespModel := hookCommon.TransformResponseInputModel{
			FailedTriggerResponses: []bitriseapi.TriggerAPIResponseModel{
				{
					Status:    "error",
					Message:   "some error happened",
					Service:   "bitrise",
					AppSlug:   "app-slug",
					BuildSlug: "build-slug",
				},
			},
		}

		resp := provider.TransformResponse(baseRespModel)
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: `Results:
*[!] Failed Triggers*:
* {Status:error Message:some error happened Service:bitrise AppSlug:app-slug BuildSlug:build-slug}`,
			},
			HTTPStatusCode: 200,
		}, resp)
	}

	t.Log("Single error")
	{
		baseRespModel := hookCommon.TransformResponseInputModel{
			Errors: []string{"a single error"},
		}

		resp := provider.TransformResponse(baseRespModel)
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: `Results:
*[!] Errors*:
* a single error`,
			},
			HTTPStatusCode: 200,
		}, resp)
	}

	t.Log("Multiple errors")
	{
		baseRespModel := hookCommon.TransformResponseInputModel{
			Errors: []string{"first error", "Second Error"},
		}

		resp := provider.TransformResponse(baseRespModel)
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: `Results:
*[!] Errors*:
* first error
* Second Error`,
			},
			HTTPStatusCode: 200,
		}, resp)
	}
}

func Test_HookProvider_TransformErrorMessageResponse(t *testing.T) {
	provider := HookProvider{}

	{
		resp := provider.TransformErrorMessageResponse("my Err msg")
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: "*[!] Error*: my Err msg",
			},
			HTTPStatusCode: 200,
		}, resp)
	}
}

func Test_HookProvider_TransformSuccessMessageResponse(t *testing.T) {
	provider := HookProvider{}

	{
		resp := provider.TransformSuccessMessageResponse("my Success msg")
		require.Equal(t, hookCommon.TransformResponseModel{
			Data: OutgoingWebhookRespModel{
				Text: "my Success msg",
			},
			HTTPStatusCode: 200,
		}, resp)
	}
}
