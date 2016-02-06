package bitbucketv2

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
			"User-Agent":  {"Bitbucket-Webhooks/2.0"},
			"X-Event-Key": {"repo:push"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.True(t, hookCheckResult.IsSupportedByProvider)
		require.NoError(t, hookCheckResult.CantTransformReason)
	}

	t.Log("Issue create event (unsupported event) - should not transform, should skip")
	{
		header := http.Header{
			"User-Agent":  {"Bitbucket-Webhooks/2.0"},
			"X-Event-Key": {"issue:create"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.True(t, hookCheckResult.IsSupportedByProvider)
		require.EqualError(t, hookCheckResult.CantTransformReason, "Unsupported Bitbucket hook event type: issue:create")
	}

	t.Log("Not a BitbucketV2 style webhook")
	{
		header := http.Header{
			"User-Agent":  {"Bitbucket-Webhooks/1.0"},
			"X-Event-Key": {"repo:push"},
		}
		hookCheckResult := provider.HookCheck(header)
		require.False(t, hookCheckResult.IsSupportedByProvider)
		require.NoError(t, hookCheckResult.CantTransformReason)
	}
}
