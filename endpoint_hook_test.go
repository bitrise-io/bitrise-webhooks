package main

import (
	"net/http"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/providers/bitbucketv2"
	"github.com/bitrise-io/bitrise-webhooks/providers/github"
	"github.com/stretchr/testify/require"
)

func Test_selectProvider(t *testing.T) {
	t.Log("Unsupported")
	{
		provider, isCantTransform := selectProvider(http.Header{})
		require.Nil(t, provider)
		require.False(t, isCantTransform)
	}
	{
		header := http.Header{
			"X-Github-Event": {"push"},
		}
		provider, isCantTransform := selectProvider(header)
		require.Nil(t, provider)
		require.False(t, isCantTransform)
	}

	t.Log("GitHub - push - json")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
			"Content-Type":        {"application/json"},
		}
		provider, isCantTransform := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, github.HookProvider{}, *provider)
		require.False(t, isCantTransform)
	}
	t.Log("GitHub - push - x-www-form-urlencoded")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
			"Content-Type":        {"application/x-www-form-urlencoded"},
		}
		provider, isCantTransform := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, github.HookProvider{}, *provider)
		require.False(t, isCantTransform)
	}
	t.Log("GitHub - pull request - json")
	{
		header := http.Header{
			"X-Github-Event": {"pull_request"},
			"Content-Type":        {"application/json"},
		}

		provider, isCantTransform := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, github.HookProvider{}, *provider)
		require.False(t, isCantTransform)
	}
	t.Log("GitHub - pull request - x-www-form-urlencoded")
	{
		header := http.Header{
			"X-Github-Event": {"pull_request"},
			"Content-Type":        {"application/x-www-form-urlencoded"},
		}

		provider, isCantTransform := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, github.HookProvider{}, *provider)
		require.False(t, isCantTransform)
	}

	// --- Bitbucket ---
	t.Log("Bitbucket-V2 - push")
	{
		header := http.Header{
			"HTTP_USER_AGENT": {"Bitbucket-Webhooks/2.0"},
			"X-Event-Key":     {"repo:push"},
		}
		provider, isCantTransform := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, bitbucketv2.HookProvider{}, *provider)
		require.False(t, isCantTransform)
	}
	// TODO: tests for Bitbucket V2
}
