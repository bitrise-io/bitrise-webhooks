package bitriseapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildTriggerURL(t *testing.T) {
	t.Log("Endpoint URL doesn't end with /")
	{
		url, err := BuildTriggerURL("https://app.bitrise.io", "a..............b")
		require.NoError(t, err)
		require.Equal(t, "https://app.bitrise.io/app/a..............b/build/start.json", url.String())
	}

	t.Log("Endpoint URL ends with /")
	{
		url, err := BuildTriggerURL("https://app.bitrise.io/", "a..............b")
		require.NoError(t, err)
		require.Equal(t, "https://app.bitrise.io/app/a..............b/build/start.json", url.String())
	}
}

func Test_TriggerAPIParamsModel_Validate(t *testing.T) {
	t.Log("Empty params")
	{
		triggerParams := TriggerAPIParamsModel{
			BuildParams: BuildParamsModel{},
		}

		err := triggerParams.Validate()
		require.EqualError(t, err, "Missing Branch, Tag and WorkflowID parameters - at least one of these is required")
	}

	t.Log("Minimal valid, with branch")
	{
		triggerParams := TriggerAPIParamsModel{
			BuildParams: BuildParamsModel{
				Branch: "develop",
			},
		}

		err := triggerParams.Validate()
		require.NoError(t, err)
	}

	t.Log("Minimal valid, with workflow")
	{
		triggerParams := TriggerAPIParamsModel{
			BuildParams: BuildParamsModel{
				WorkflowID: "my-wf",
			},
		}

		err := triggerParams.Validate()
		require.NoError(t, err)
	}

	t.Log("Minimal valid, with tag")
	{
		triggerParams := TriggerAPIParamsModel{
			BuildParams: BuildParamsModel{
				Tag: "v0.0.2",
			},
		}

		err := triggerParams.Validate()
		require.NoError(t, err)
	}
}

func TestTriggerBuild(t *testing.T) {
	url, err := BuildTriggerURL("https://app.bitrise.io", "app-slug")
	require.NoError(t, err)

	t.Log("Empty trigger api params (invalid)")
	{
		triggerParams := TriggerAPIParamsModel{
			BuildParams: BuildParamsModel{},
		}

		apiResponse, isSuccess, err := TriggerBuild(url, "api-token", triggerParams, true)
		require.Equal(t, false, isSuccess)
		require.EqualError(t, err, "TriggerBuild: build trigger parameter invalid: Missing Branch, Tag and WorkflowID parameters - at least one of these is required")
		require.Equal(t, TriggerAPIResponseModel{}, apiResponse)
	}

	t.Log("Should be able to process - in log-only mode, no request made - branch only")
	{
		triggerParams := TriggerAPIParamsModel{
			BuildParams: BuildParamsModel{
				Branch: "develop",
			},
		}

		apiResponse, isSuccess, err := TriggerBuild(url, "api-token", triggerParams, true)
		require.Equal(t, true, isSuccess)
		require.NoError(t, err)
		require.Equal(t, TriggerAPIResponseModel{
			Status:  "ok",
			Message: "LOG ONLY MODE",
		}, apiResponse)
	}

	t.Log("Should be able to process - in log-only mode, no request made - workflowID only")
	{
		triggerParams := TriggerAPIParamsModel{
			BuildParams: BuildParamsModel{
				WorkflowID: "my-wf",
			},
		}

		apiResponse, isSuccess, err := TriggerBuild(url, "api-token", triggerParams, true)
		require.Equal(t, true, isSuccess)
		require.NoError(t, err)
		require.Equal(t, TriggerAPIResponseModel{
			Status:  "ok",
			Message: "LOG ONLY MODE",
		}, apiResponse)
	}

	t.Log("Should be able to process - in log-only mode, no request made - tag only")
	{
		triggerParams := TriggerAPIParamsModel{
			BuildParams: BuildParamsModel{
				Tag: "v0.0.2",
			},
		}

		apiResponse, isSuccess, err := TriggerBuild(url, "api-token", triggerParams, true)
		require.Equal(t, true, isSuccess)
		require.NoError(t, err)
		require.Equal(t, TriggerAPIResponseModel{
			Status:  "ok",
			Message: "LOG ONLY MODE",
		}, apiResponse)
	}
}
