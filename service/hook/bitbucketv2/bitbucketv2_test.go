package bitbucketv2

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_detectContentTypeUserAgentAndEventKey(t *testing.T) {
	t.Log("Push event - should handle")
	{
		header := http.Header{
			"X-Event-Key":  {"repo:push"},
			"Content-Type": {"application/json"},
			"User-Agent":   {"Bitbucket-Webhooks/2.0"},
		}
		contentType, userAgent, eventKey, err := detectContentTypeUserAgentAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "Bitbucket-Webhooks/2.0", userAgent)
		require.Equal(t, "repo:push", eventKey)
	}

	t.Log("Unsupported event - will be handled in Transform")
	{
		header := http.Header{
			"X-Event-Key":  {"issue:create"},
			"Content-Type": {"application/json"},
			"User-Agent":   {"Bitbucket-Webhooks/2.0"},
		}
		contentType, userAgent, eventKey, err := detectContentTypeUserAgentAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "Bitbucket-Webhooks/2.0", userAgent)
		require.Equal(t, "issue:create", eventKey)
	}

	t.Log("Missing X-Event-Key header")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
			"User-Agent":   {"Bitbucket-Webhooks/2.0"},
		}
		contentType, userAgent, eventKey, err := detectContentTypeUserAgentAndEventKey(header)
		require.EqualError(t, err, "Issue with X-Event-Key Header: No value found in HEADER for the key: X-Event-Key")
		require.Equal(t, "", contentType)
		require.Equal(t, "", userAgent)
		require.Equal(t, "", eventKey)
	}

	t.Log("Missing Content-Type header")
	{
		header := http.Header{
			"X-Event-Key": {"repo:push"},
			"User-Agent":  {"Bitbucket-Webhooks/2.0"},
		}
		contentType, userAgent, eventKey, err := detectContentTypeUserAgentAndEventKey(header)
		require.EqualError(t, err, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
		require.Equal(t, "", contentType)
		require.Equal(t, "", userAgent)
		require.Equal(t, "", eventKey)
	}

	t.Log("Missing User-Agent header")
	{
		header := http.Header{
			"X-Event-Key":  {"repo:push"},
			"Content-Type": {"application/json"},
		}
		contentType, userAgent, eventKey, err := detectContentTypeUserAgentAndEventKey(header)
		require.EqualError(t, err, "Issue with User-Agent Header: No value found in HEADER for the key: User-Agent")
		require.Equal(t, "", contentType)
		require.Equal(t, "", userAgent)
		require.Equal(t, "", eventKey)
	}
}
