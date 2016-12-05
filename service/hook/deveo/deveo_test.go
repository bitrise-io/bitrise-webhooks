package deveo

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/stretchr/testify/require"
)

const (
	sampleCodePushData = `{
  "ref": "refs/heads/master",
  "deleted": false,
  "commits": [{
    "distinct": true,
    "id": "83b86e5f286f546dc5a4a58db66ceef44460c85e",
    "message": "re-structuring Hook Providers, with added tests"
  }]
}`

	sampleTagPushData = `{
  "ref": "refs/tags/v0.0.2",
  "deleted": false,
  "commits": [{
    "distinct": true,
    "id": "2e197ebd2330183ae11338151cf3a75db0c23c92",
    "message": "generalize Push Event (previously Code Push)\n\nwe'll handle the Tag Push too, so related codes are changed to reflect this (removed code from CodePush - e.g. CodePushEventModel -> PushEventModel)"
  }]
}`
)

func Test_detectContentTypeAndEventID(t *testing.T) {
	t.Log("Push event - should handle")
	{
		header := http.Header{
			"X-Deveo-Event": {"push"},
			"Content-Type":  {"application/json"},
		}
		contentType, deveoEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "push", deveoEvent)
	}

	t.Log("Unsupported Deveo event - will be handled in Transform")
	{
		header := http.Header{
			"X-Deveo-Event": {"label"},
			"Content-Type":  {"application/json"},
		}
		contentType, deveoEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "label", deveoEvent)
	}

	t.Log("Missing X-Deveo-Event header")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
		}
		contentType, deveoEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "Issue with X-Deveo-Event Header: No value found in HEADER for the key: X-Deveo-Event")
		require.Equal(t, "", contentType)
		require.Equal(t, "", deveoEvent)
	}

	t.Log("Missing Content-Type")
	{
		header := http.Header{
			"X-Deveo-Event": {"push"},
		}
		contentType, deveoEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
		require.Equal(t, "", contentType)
		require.Equal(t, "", deveoEvent)
	}
}

func Test_transformPushEvent(t *testing.T) {
	t.Log("Do Transform - Code Push")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      true,
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Do Transform - Tag Push")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      true,
					CommitHash:    "2e197ebd2330183ae11338151cf3a75db0c23c92",
					CommitMessage: "generalize Push Event (previously Code Push)",
				},
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.2",
					CommitHash:    "2e197ebd2330183ae11338151cf3a75db0c23c92",
					CommitMessage: "generalize Push Event (previously Code Push)",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Not Distinct Head Commit - should still trigger a build (e.g. this can happen if you rebase-merge a branch, without creating a merge commit)")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      false,
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Tag - Not Distinct Head Commit - should still trigger a build")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      false,
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.2",
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Missing Commit Hash")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      true,
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "Missing commit hash")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Missing Commit Hash - Tag")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      true,
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.EqualError(t, hookTransformResult.Error, "Missing commit hash")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("This is a 'deleted' event")
	{
		codePush := PushEventModel{
			Deleted: true,
			Ref:     "refs/heads/master",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      true,
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "This is a 'Deleted' event, no build can be started")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("This is a 'deleted' event - Tag")
	{
		tagPush := PushEventModel{
			Deleted: true,
			Ref:     "refs/tags/v0.0.2",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      true,
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "This is a 'Deleted' event, no build can be started")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Not a head nor a tag ref")
	{
		codePush := PushEventModel{
			Ref: "refs/not/head",
			Commits: []CommitModel{
				CommitModel{
					Distinct:      true,
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Ref (refs/not/head) is not a head nor a tag ref")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Unsuported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":  {"not/supported"},
				"X-Deveo-Event": {"ping"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("Unsupported event type - should error")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":  {"application/json"},
				"X-Deveo-Event": {"label"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Unsupported Deveo Webhook event: label")
	}

	t.Log("Push Event - should not be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":  {"application/json"},
				"X-Deveo-Event": {"push"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":  {"application/json"},
				"X-Deveo-Event": {"push"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Code Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Deveo-Event": {"push"},
				"Content-Type":  {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Tag Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Deveo-Event": {"push"},
				"Content-Type":  {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleTagPushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.2",
					CommitHash:    "2e197ebd2330183ae11338151cf3a75db0c23c92",
					CommitMessage: "generalize Push Event (previously Code Push)\n\nwe'll handle the Tag Push too, so related codes are changed to reflect this (removed code from CodePush - e.g. CodePushEventModel -> PushEventModel)",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}
}
