package bitbucketv2

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
"push": {
	"changes": [
		{
			"new": {
				"name": "master",
				"type": "branch",
				"target": {
					"type": "commit",
					"message": "auto-test",
					"hash": "966d0bfe79b80f97268c2f6bb45e65e79ef09b31"
				}
			}
		},
		{
			"new": {
				"name": "test",
				"type": "branch",
				"target": {
					"type": "commit",
					"message": "auto-test 2",
					"hash": "19934139a2cf799bbd0f5061ab02e4760902e93f"
				}
			}
		}
	]
}
}`
)

func Test_detectContentTypeUserAgentAndEventKey(t *testing.T) {
	t.Log("Push event - should handle")
	{
		header := http.Header{
			"X-Event-Key":      {"repo:push"},
			"Content-Type":     {"application/json"},
			"X-Attempt-Number": {"1"},
		}
		contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "repo:push", eventKey)
		require.Equal(t, "1", attemptNum)
	}

	t.Log("Unsupported event - will be handled in Transform")
	{
		header := http.Header{
			"X-Event-Key":      {"issue:create"},
			"Content-Type":     {"application/json"},
			"X-Attempt-Number": {"2"},
		}
		contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "issue:create", eventKey)
		require.Equal(t, "2", attemptNum)
	}

	t.Log("Missing X-Event-Key header")
	{
		header := http.Header{
			"Content-Type":     {"application/json"},
			"X-Attempt-Number": {"1"},
		}
		contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(header)
		require.EqualError(t, err, "Issue with X-Event-Key Header: No value found in HEADER for the key: X-Event-Key")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventKey)
		require.Equal(t, "", attemptNum)
	}

	t.Log("Missing Content-Type header")
	{
		header := http.Header{
			"X-Event-Key":      {"repo:push"},
			"X-Attempt-Number": {"1"},
		}
		contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(header)
		require.EqualError(t, err, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventKey)
		require.Equal(t, "", attemptNum)
	}

	t.Log("Missing X-Attempt-Number header")
	{
		header := http.Header{
			"X-Event-Key":  {"repo:push"},
			"Content-Type": {"application/json"},
		}
		contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(header)
		require.EqualError(t, err, "Issue with X-Attempt-Number Header: No value found in HEADER for the key: X-Attempt-Number")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventKey)
		require.Equal(t, "", attemptNum)
	}
}

func Test_transformCodePushEvent(t *testing.T) {
	t.Log("Do Transform - single change")
	{
		codePush := CodePushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "branch",
							Name: "master",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
								CommitMessage: "auto-test",
							},
						},
					},
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					CommitMessage: "auto-test",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Do Transform - multiple changes")
	{
		codePush := CodePushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "branch",
							Name: "master",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
								CommitMessage: "auto-test",
							},
						},
					},
					{
						ChangeNewItem: ChangeItemModel{
							Type: "branch",
							Name: "test",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
								CommitMessage: "auto-test 2",
							},
						},
					},
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					CommitMessage: "auto-test",
					Branch:        "master",
				},
			},
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
					CommitMessage: "auto-test 2",
					Branch:        "test",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("One of the changes is not a type=branch change")
	{
		codePush := CodePushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "tag",
							Name: "1.0.0",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
								CommitMessage: "auto-test",
							},
						},
					},
					{
						ChangeNewItem: ChangeItemModel{
							Type: "branch",
							Name: "test",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
								CommitMessage: "auto-test 2",
							},
						},
					},
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
					CommitMessage: "auto-test 2",
					Branch:        "test",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("One of the changes is not a type=commit change")
	{
		codePush := CodePushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "branch",
							Name: "master",
							Target: ChangeItemTargetModel{
								Type:          "not-commit",
								CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
								CommitMessage: "auto-test",
							},
						},
					},
					{
						ChangeNewItem: ChangeItemModel{
							Type: "branch",
							Name: "test",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
								CommitMessage: "auto-test 2",
							},
						},
					},
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
					CommitMessage: "auto-test 2",
					Branch:        "test",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Not a Branch code push event")
	{
		codePush := CodePushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "tag",
							Name: "1.0.0",
						},
					},
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "'changes' specified in the webhook, but none can be transformed into a build. Collected errors: [Not a type=branch change. Type was: tag]")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Not a 'Commit' type change")
	{
		codePush := CodePushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "branch",
							Name: "master",
							Target: ChangeItemTargetModel{
								Type: "unsupported-type",
							},
						},
					},
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "'changes' specified in the webhook, but none can be transformed into a build. Collected errors: [Target: Not a type=commit change. Type was: unsupported-type]")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("It's a re-try (X-Attempt-Number >= 2) - skip")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"repo:push"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"2"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "No retry is supported (X-Attempt-Number: 2)")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Unsupported Event Type")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"not:supported"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"1"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "X-Event-Key is not supported: not:supported")
	}

	t.Log("Unsupported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"repo:push"},
				"Content-Type":     {"not/supported"},
				"X-Attempt-Number": {"1"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"repo:push"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"1"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Test with Sample Code Push data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"repo:push"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"1"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					CommitMessage: "auto-test",
					Branch:        "master",
				},
			},
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "19934139a2cf799bbd0f5061ab02e4760902e93f",
					CommitMessage: "auto-test 2",
					Branch:        "test",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("X-Attempt-Number=1 - OK")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"repo:push"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"1"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					CommitMessage: "auto-test",
					Branch:        "master",
				},
			},
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "19934139a2cf799bbd0f5061ab02e4760902e93f",
					CommitMessage: "auto-test 2",
					Branch:        "test",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("X-Attempt-Number=2 - SKIP")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"repo:push"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"2"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "No retry is supported (X-Attempt-Number: 2)")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}
}
