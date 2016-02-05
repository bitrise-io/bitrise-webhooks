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
		provider, cantTransformReason := selectProvider(http.Header{})
		require.Nil(t, provider)
		require.NoError(t, cantTransformReason)
	}
	{
		header := http.Header{
			"X-Github-Event": {"push"},
		}
		provider, cantTransformReason := selectProvider(header)
		require.Nil(t, provider)
		require.NoError(t, cantTransformReason)
	}

	t.Log("GitHub - push - json")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
			"Content-Type":   {"application/json"},
		}
		provider, cantTransformReason := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, github.HookProvider{}, *provider)
		require.NoError(t, cantTransformReason)
	}
	t.Log("GitHub - push - x-www-form-urlencoded")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
			"Content-Type":   {"application/x-www-form-urlencoded"},
		}
		provider, cantTransformReason := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, github.HookProvider{}, *provider)
		require.NoError(t, cantTransformReason)
	}
	t.Log("GitHub - pull request - json")
	{
		header := http.Header{
			"X-Github-Event": {"pull_request"},
			"Content-Type":   {"application/json"},
		}

		provider, cantTransformReason := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, github.HookProvider{}, *provider)
		require.NoError(t, cantTransformReason)
	}
	t.Log("GitHub - pull request - x-www-form-urlencoded")
	{
		header := http.Header{
			"X-Github-Event": {"pull_request"},
			"Content-Type":   {"application/x-www-form-urlencoded"},
		}

		provider, cantTransformReason := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, github.HookProvider{}, *provider)
		require.NoError(t, cantTransformReason)
	}

	// --- Bitbucket ---
	t.Log("Bitbucket-V2 - push")
	{
		header := http.Header{
			"User-Agent":  {"Bitbucket-Webhooks/2.0"},
			"X-Event-Key": {"repo:push"},
		}
		provider, cantTransformReason := selectProvider(header)
		require.NotNil(t, provider)
		require.IsType(t, bitbucketv2.HookProvider{}, *provider)
		require.NoError(t, cantTransformReason)
	}
	// TODO: tests for Bitbucket V2
}
