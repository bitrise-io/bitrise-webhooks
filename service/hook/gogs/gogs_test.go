package gogs

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/stretchr/testify/require"
)

func Test_detectContentTypeAndEventID(t *testing.T) {
	t.Log("Code Push event")
	{
		header := http.Header{
			"X-Gogs-Event": {"push"},
			"Content-Type": {"application/json"},
		}
		contentType, eventID, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "push", eventID)
	}

	t.Log("Missing X-Gogs-Event header")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
		}
		contentType, eventID, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "No X-Gogs-Event Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventID)
	}

	t.Log("Missing Content-Type")
	{
		header := http.Header{
			"X-Gogs-Event": {"push"},
		}
		contentType, eventID, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "No Content-Type Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventID)
	}
}

func Test_transformCodePushEvent(t *testing.T) {
	t.Log("Do Transform - single commit")
	{
		codePush := PushEventModel{
			Ref:         "refs/heads/master",
			CheckoutSHA: "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
			Commits: []CommitModel{
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Do Transform - multiple commits - CheckoutSHA match should trigger the build")
	{
		codePush := PushEventModel{
			Ref:         "refs/heads/master",
			CheckoutSHA: "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
			Commits: []CommitModel{
				{
					CommitHash:    "7782203aaf0daabbd245ec0370c751eac6a4eb55",
					CommitMessage: `switch to three component versions`,
				},
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
				},
				{
					CommitHash:    "ef77f9dba498f335e2e7078bdb52f9e11c214c58",
					CommitMessage: `get version : three component version`,
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("No commit matches CheckoutSHA")
	{
		codePush := PushEventModel{
			Ref:         "refs/heads/master",
			CheckoutSHA: "checkout-sha",
			Commits: []CommitModel{
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "The commit specified by 'after' was not included in the 'commits' array - no match found")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a head ref")
	{
		codePush := PushEventModel{
			Ref:         "refs/not/head",
			CheckoutSHA: "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
			Commits: []CommitModel{
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "Ref (refs/not/head) is not a head ref")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Is a tag ref")
	{
		codePush := PushEventModel{
			Ref:         "refs/tags/1.0.0",
			CheckoutSHA: "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
		}
		hookTransformResult := transformPushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	const sampleCodePushData = `{
  "secret": "",
  "ref": "refs/heads/develop",
  "after": "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
  "commits": [
    {
      "id": "29da60ce2c47a6696bc82f2e6ec4a075695eb7c3",
      "message": "first commit message"
    },
    {
      "id": "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
      "message": "second commit message"
    }
  ]
}`

	const sampleTagPushData = `{
  "secret": "",
  "ref": "v1.12",
  "ref_type": "tag",
  "id":"commithash",
  "message":"Simple message"
}`

	const sampleBranchCreatePushData = `{
  "secret": "",
  "ref": "mybranch",
  "ref_type": "branch"
}`

	t.Log("Code Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gogs-Event": {"push"},
				"Content-Type": {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
					CommitMessage: "second commit message",
					Branch:        "develop",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Tag Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gogs-Event": {"create"},
				"Content-Type": {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleTagPushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v1.12",
					CommitHash:    "commithash",
					CommitMessage: "Simple message",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Branch Create - should be ignored")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gogs-Event": {"create"},
				"Content-Type": {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleBranchCreatePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Not a tag create event - ignoring")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Unsupported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gogs-Event": {"push"},
				"Content-Type": {"not/supported"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gogs-Event": {"push"},
				"Content-Type": {"application/json"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}
}
