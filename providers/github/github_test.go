package github

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_HookProvider_HookCheck(t *testing.T) {
	provider := HookProvider{}

	t.Log("Push event - should handle")
	{
		header := http.Header{
			"HTTP_X_GITHUB_EVENT": {"push"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.True(t, hookCheckResult.IsSupportedByProvider)
		require.False(t, hookCheckResult.IsCantTransform)
	}

	t.Log("Pull Request event - should handle")
	{
		header := http.Header{
			"HTTP_X_GITHUB_EVENT": {"pull_request"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.True(t, hookCheckResult.IsSupportedByProvider)
		require.False(t, hookCheckResult.IsCantTransform)
	}

	t.Log("Ping event (unsupported GH event) - should not transform, should skip")
	{
		header := http.Header{
			"HTTP_X_GITHUB_EVENT": {"ping"},
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
}
