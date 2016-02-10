package github

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/go-utils/pointers"
	"github.com/stretchr/testify/require"
)

const (
	sampleCodePushData = `{
  "ref": "refs/heads/master",
  "deleted": false,
  "head_commit": {
    "distinct": true,
    "id": "83b86e5f286f546dc5a4a58db66ceef44460c85e",
    "message": "re-structuring Hook Providers, with added tests"
  }
}`

	samplePullRequestData = `{
  "action": "opened",
  "number": 12,
  "pull_request": {
    "head": {
      "ref": "master",
      "sha": "83b86e5f286f546dc5a4a58db66ceef44460c85e"
    },
    "title": "PR test",
    "body": "PR text body",
    "merged": false,
    "mergeable": true
  }
}`
)

func Test_detectContentTypeAndEventID(t *testing.T) {
	t.Log("Push event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "push", ghEvent)
	}

	t.Log("Pull Request event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"pull_request"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "pull_request", ghEvent)
	}

	t.Log("Ping event")
	{
		header := http.Header{
			"X-Github-Event": {"ping"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "ping", ghEvent)
	}

	t.Log("Unsupported GH event - will be handled in Transform")
	{
		header := http.Header{
			"X-Github-Event": {"label"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "label", ghEvent)
	}

	t.Log("Missing X-Github-Event header")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "Issue with X-Github-Event Header: No value found in HEADER for the key: X-Github-Event")
		require.Equal(t, "", contentType)
		require.Equal(t, "", ghEvent)
	}

	t.Log("Missing Content-Type")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
		require.Equal(t, "", contentType)
		require.Equal(t, "", ghEvent)
	}
}

func Test_HookProvider_Transform(t *testing.T) {
	provider := HookProvider{}

	t.Log("Ping - should be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"ping"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Ping event received")
	}

	t.Log("Unsuported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"not/supported"},
				"X-Github-Event": {"ping"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("Unsupported event type - should error")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"label"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Unsupported GitHub Webhook event: label")
	}

	t.Log("Code Push - should not be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"push"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Pull Request - should not be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"pull_request"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"push"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Code Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"push"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.Transform(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, bitriseapi.TriggerAPIParamsModel{
			CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			CommitMessage: "re-structuring Hook Providers, with added tests",
			Branch:        "master",
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Pull Request - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"pull_request"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePullRequestData)),
		}
		hookTransformResult := provider.Transform(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, bitriseapi.TriggerAPIParamsModel{
			CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			CommitMessage: "PR test\n\nPR text body",
			Branch:        "master",
			PullRequestID: pointers.NewIntPtr(12),
		}, hookTransformResult.TriggerAPIParams)
	}
}

func Test_transformCodePushEvent(t *testing.T) {
	t.Log("Not Distinct Head Commit")
	{
		codePush := CodePushEventModel{
			HeadCommit: CommitModel{Distinct: false},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Head Commit is not Distinct")
	}

	t.Log("This is a 'deleted' event")
	{
		codePush := CodePushEventModel{
			HeadCommit: CommitModel{
				Distinct: true,
			},
			Deleted: true,
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "This is a 'Deleted' event, no build can be started")
	}

	t.Log("Not a head ref")
	{
		codePush := CodePushEventModel{
			Ref:        "refs/pull/a",
			HeadCommit: CommitModel{Distinct: true},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Ref (refs/pull/a) is not a head ref")
	}

	t.Log("Do Transform")
	{
		codePush := CodePushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, bitriseapi.TriggerAPIParamsModel{
			CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			CommitMessage: "re-structuring Hook Providers, with added tests",
			Branch:        "master",
		}, hookTransformResult.TriggerAPIParams)
	}
}

func Test_transformPullRequestEvent(t *testing.T) {
	t.Log("Unsupported Pull Request action")
	{
		pullRequest := PullRequestEventModel{
			Action: "labeled",
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request action doesn't require a build: labeled")
	}

	t.Log("Empty Pull Request action")
	{
		pullRequest := PullRequestEventModel{}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "No Pull Request action specified")
	}

	t.Log("Already Merged")
	{
		pullRequest := PullRequestEventModel{
			Action: "opened",
			PullRequestInfo: PullRequestInfoModel{
				Merged: true,
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request already merged")
	}

	t.Log("Not Mergeable")
	{
		pullRequest := PullRequestEventModel{
			Action: "reopened",
			PullRequestInfo: PullRequestInfoModel{
				Merged:    false,
				Mergeable: pointers.NewBoolPtr(false),
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request is not mergeable")
	}

	t.Log("Mergeable: not yet decided")
	{
		pullRequest := PullRequestEventModel{
			Action:        "synchronize",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Merged:    false,
				Mergeable: nil,
				BranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				},
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.False(t, hookTransformResult.ShouldSkip)
		require.NoError(t, hookTransformResult.Error)
		require.Equal(t, bitriseapi.TriggerAPIParamsModel{
			CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			CommitMessage: "PR test",
			Branch:        "master",
			PullRequestID: pointers.NewIntPtr(12),
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Mergeable: true")
	{
		pullRequest := PullRequestEventModel{
			Action:        "synchronize",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Merged:    false,
				Mergeable: pointers.NewBoolPtr(true),
				BranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				},
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, bitriseapi.TriggerAPIParamsModel{
			CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			CommitMessage: "PR test",
			Branch:        "master",
			PullRequestID: pointers.NewIntPtr(12),
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Pull Request - Title & Body")
	{
		pullRequest := PullRequestEventModel{
			Action:        "opened",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: pointers.NewBoolPtr(true),
				BranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				},
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, bitriseapi.TriggerAPIParamsModel{
			CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			CommitMessage: "PR test\n\nPR text body",
			Branch:        "master",
			PullRequestID: pointers.NewIntPtr(12),
		}, hookTransformResult.TriggerAPIParams)
	}
}
