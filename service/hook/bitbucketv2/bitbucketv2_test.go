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
				CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
				CommitMessage: "auto-test",
				Branch:        "master",
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
				CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
				CommitMessage: "auto-test",
				Branch:        "master",
			},
			{
				CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
				CommitMessage: "auto-test 2",
				Branch:        "test",
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
				CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
				CommitMessage: "auto-test 2",
				Branch:        "test",
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
				CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
				CommitMessage: "auto-test 2",
				Branch:        "test",
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

func Test_HookProvider_Transform(t *testing.T) {
	provider := HookProvider{}

	t.Log("Unsupported Event Type")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"not:supported"},
				"Content-Type": {"application/json"},
				"User-Agent":   {"Bitbucket-Webhooks/2.0"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "X-Event-Key is not supported: not:supported")
	}

	t.Log("Unsupported User-Agent")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"repo:push"},
				"Content-Type": {"application/json"},
				"User-Agent":   {"not/supported"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "User-Agent is not supported: not/supported")
	}

	t.Log("Unsupported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"repo:push"},
				"Content-Type": {"not/supported"},
				"User-Agent":   {"Bitbucket-Webhooks/2.0"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"repo:push"},
				"Content-Type": {"application/json"},
				"User-Agent":   {"Bitbucket-Webhooks/2.0"},
			},
		}
		hookTransformResult := provider.Transform(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Test with Sample Code Push data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":  {"repo:push"},
				"Content-Type": {"application/json"},
				"User-Agent":   {"Bitbucket-Webhooks/2.0"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.Transform(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
				CommitMessage: "auto-test",
				Branch:        "master",
			},
			{
				CommitHash:    "19934139a2cf799bbd0f5061ab02e4760902e93f",
				CommitMessage: "auto-test 2",
				Branch:        "test",
			},
		}, hookTransformResult.TriggerAPIParams)
	}
}
