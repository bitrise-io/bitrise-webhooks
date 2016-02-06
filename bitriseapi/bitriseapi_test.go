package bitriseapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildTriggerURL(t *testing.T) {
	t.Log("Endpoint URL doesn't end with /")
	{
		url, err := BuildTriggerURL("https://www.bitrise.io", "a..............b")
		require.NoError(t, err)
		require.Equal(t, "https://www.bitrise.io/app/a..............b/build/start.json", url.String())
	}

	t.Log("Endpoint URL ends with /")
	{
		url, err := BuildTriggerURL("https://www.bitrise.io/", "a..............b")
		require.NoError(t, err)
		require.Equal(t, "https://www.bitrise.io/app/a..............b/build/start.json", url.String())
	}
}
