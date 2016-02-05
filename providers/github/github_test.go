package github

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/stretchr/testify/require"
)

func Test_HookProvider_HookCheck(t *testing.T) {
	provider := HookProvider{}

	t.Log("Push event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
			"Content-Type":        {"application/json"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.True(t, hookCheckResult.IsSupportedByProvider)
		require.False(t, hookCheckResult.IsCantTransform)
	}

	t.Log("Pull Request event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"pull_request"},
			"Content-Type":        {"application/json"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.True(t, hookCheckResult.IsSupportedByProvider)
		require.False(t, hookCheckResult.IsCantTransform)
	}

	t.Log("Ping event (unsupported GH event) - should not transform, should skip")
	{
		header := http.Header{
			"X-Github-Event": {"ping"},
			"Content-Type":        {"application/json"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.True(t, hookCheckResult.IsSupportedByProvider)
		require.True(t, hookCheckResult.IsCantTransform)
	}

	t.Log("Not a GitHub style webhook")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.False(t, hookCheckResult.IsSupportedByProvider)
		require.False(t, hookCheckResult.IsCantTransform)
	}

	t.Log("Missing Content-Type")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.False(t, hookCheckResult.IsSupportedByProvider)
		require.False(t, hookCheckResult.IsCantTransform)
	}
}

func Test_HookProvider_Transform(t *testing.T) {
	provider := HookProvider{}

	t.Log("Code Push")
	{
		request := http.Request{
			Header: http.Header{"X-Github-Event": {"push"}},
			Body:   ioutil.NopCloser(strings.NewReader("hi")),
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
	}

	t.Log("Pull Request")
	{
		request := http.Request{
			Header: http.Header{"X-Github-Event": {"pull_request"}},
			Body:   ioutil.NopCloser(strings.NewReader("hi")),
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{"X-Github-Event": {"push"}},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
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
		require.Equal(t, bitriseapi.TriggerAPIParamsModel{
			CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			CommitMessage: "re-structuring Hook Providers, with added tests",
			Branch:        "master",
		}, hookTransformResult.TriggerAPIParams)
		require.False(t, hookTransformResult.ShouldSkip)
	}
}
