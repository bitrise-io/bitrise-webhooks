package bitbucketv2

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/go-utils/pointers"
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
},
"repository": {
  "owner": {},
  "full_name": "test/testrepo",
  "is_private": true,
  "type": "repository",
  "scm": "git"
}
}`

	sampleMercurialCodePushData = `{
"push": {
	"changes": [
		{
			"new": {
				"name": "master",
				"type": "named_branch",
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
				"type": "named_branch",
				"target": {
					"type": "commit",
					"message": "auto-test 2",
					"hash": "19934139a2cf799bbd0f5061ab02e4760902e93f"
				}
			}
		}
	]
},
"repository": {
  "owner": {},
  "full_name": "test/hg-testrepo",
  "is_private": true,
  "type": "repository",
  "scm": "hg"
}
}`

	sampleTagPushData = `{
"push": {
	"changes": [
		{
			"new": {
				"type": "tag",
				"name": "v0.0.2",
				"target": {
					"type": "commit",
					"message": "auto-test",
					"hash": "966d0bfe79b80f97268c2f6bb45e65e79ef09b31"
				}
			}
		},
		{
			"new": {
				"type": "tag",
				"name": "v0.0.1",
				"target": {
					"type": "commit",
					"message": "auto-test 2",
					"hash": "19934139a2cf799bbd0f5061ab02e4760902e93f"
				}
			}
		}
	]
},
"repository": {
  "owner": {},
  "full_name": "test/testrepo",
  "is_private": true,
  "type": "repository",
  "scm": "git"
}
}`

	samplePullRequestData = `{
"pullrequest":{
  "description":"",
  "type":"pullrequest",
  "destination":{
    "commit":{
      "hash":"7b3172ca0cf8"
    },
    "branch":{
      "name":"master"
    },
    "repository":{
      "name":"prtest",
      "full_name":"birmacher/prtest"
    }
  },
  "title":"change",
  "id":1,
  "state":"OPEN",
  "source":{
    "commit":{
      "hash":"6a3451888d91"
    },
    "branch":{
      "name":"feature/test"
    },
    "repository":{
      "name":"prtest",
      "full_name":"birmacher/prtest"
    }
  }
}
}`
)

func Test_detectContentTypeAttemptNumberAndEventKey(t *testing.T) {
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

	t.Log("Pull Request create event - should handle")
	{
		header := http.Header{
			"X-Event-Key":      {"pullrequest:create"},
			"Content-Type":     {"application/json"},
			"X-Attempt-Number": {"1"},
		}
		contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "pullrequest:create", eventKey)
		require.Equal(t, "1", attemptNum)
	}

	t.Log("Pull Request update event - should handle")
	{
		header := http.Header{
			"X-Event-Key":      {"pullrequest:update"},
			"Content-Type":     {"application/json"},
			"X-Attempt-Number": {"1"},
		}
		contentType, attemptNum, eventKey, err := detectContentTypeAttemptNumberAndEventKey(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "pullrequest:update", eventKey)
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
		require.EqualError(t, err, "No X-Event-Key Header found")
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
		require.EqualError(t, err, "No Content-Type Header found")
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
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "repo:push", eventKey)
		require.Equal(t, "1", attemptNum)
	}
}

func Test_transformPushEvent(t *testing.T) {
	t.Log("Do Transform - single change - code push")
	{
		pushEvent := PushEventModel{
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
			RepositoryInfo: RepositoryInfoModel{
				Scm: "git",
			},
		}

		// OK
		{
			hookTransformResult := transformPushEvent(pushEvent)
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
			require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
		}

		// no Scm info
		pushEvent.RepositoryInfo.Scm = "invalid-scm-or-empty"
		{
			hookTransformResult := transformPushEvent(pushEvent)
			require.EqualError(t, hookTransformResult.Error, "Unsupported repository / source control type (SCM): invalid-scm-or-empty")
			require.False(t, hookTransformResult.ShouldSkip)
			require.Nil(t, hookTransformResult.TriggerAPIParams)
			require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
		}
	}

	t.Log("Do Transform - single change - tag")
	{
		tagPushEvent := PushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "tag",
							Name: "v0.0.2",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
								CommitMessage: "auto-test",
							},
						},
					},
				},
			},
			RepositoryInfo: RepositoryInfoModel{
				Scm: "git",
			},
		}
		hookTransformResult := transformPushEvent(tagPushEvent)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.2",
					CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					CommitMessage: "auto-test",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Do Transform - multiple changes - code push")
	{
		pushEvent := PushEventModel{
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
			RepositoryInfo: RepositoryInfoModel{
				Scm: "git",
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Do Transform - multiple changes - tag push")
	{
		pushEvent := PushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "tag",
							Name: "v0.0.2",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
								CommitMessage: "auto-test",
							},
						},
					},
					{
						ChangeNewItem: ChangeItemModel{
							Type: "tag",
							Name: "v0.0.1",
							Target: ChangeItemTargetModel{
								Type:          "commit",
								CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
								CommitMessage: "auto-test 2",
							},
						},
					},
				},
			},
			RepositoryInfo: RepositoryInfoModel{
				Scm: "git",
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.2",
					CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					CommitMessage: "auto-test",
				},
			},
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.1",
					CommitHash:    "178de4f94efbfa99abede5cf0f1868924222839e",
					CommitMessage: "auto-test 2",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Multiple changes, one of the changes is a not supported (type) change")
	{
		pushEvent := PushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "not-branch-nor-tag",
							Name: "name-something",
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
			RepositoryInfo: RepositoryInfoModel{
				Scm: "git",
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("One of the changes.Target is not a type=commit change")
	{
		pushEvent := PushEventModel{
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
			RepositoryInfo: RepositoryInfoModel{
				Scm: "git",
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a Branch nor Tag push event")
	{
		pushEvent := PushEventModel{
			PushInfo: PushInfoModel{
				Changes: []ChangeInfoModel{
					{
						ChangeNewItem: ChangeItemModel{
							Type: "not-branch-nor-tag",
							Name: "name-something",
						},
					},
				},
			},
			RepositoryInfo: RepositoryInfoModel{
				Scm: "git",
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
		require.EqualError(t, hookTransformResult.Error, "'changes' specified in the webhook, but none can be transformed into a build. Collected errors: [Not a type=branch nor type=tag change. Change.Type was: not-branch-nor-tag]")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a 'Commit' type change")
	{
		pushEvent := PushEventModel{
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
			RepositoryInfo: RepositoryInfoModel{
				Scm: "git",
			},
		}
		hookTransformResult := transformPushEvent(pushEvent)
		require.EqualError(t, hookTransformResult.Error, "'changes' specified in the webhook, but none can be transformed into a build. Collected errors: [Target was not a type=commit change. Type was: unsupported-type]")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_transformPullRequestEvent(t *testing.T) {
	t.Log("Empty Pull Request action")
	{
		pullRequest := PullRequestEventModel{}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request type is not supported: ")
	}

	t.Log("Invalid type")
	{
		pullRequest := PullRequestEventModel{
			PullRequestInfo: PullRequestInfoModel{
				Type: "Issue",
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request type is not supported: Issue")
	}

	t.Log("Already Merged")
	{
		pullRequest := PullRequestEventModel{
			PullRequestInfo: PullRequestInfoModel{
				Type:  "pullrequest",
				State: "MERGED",
			},
		}

		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request state doesn't require a build: MERGED")
	}

	t.Log("Already Declined")
	{
		pullRequest := PullRequestEventModel{
			PullRequestInfo: PullRequestInfoModel{
				Type:  "pullrequest",
				State: "DECLINED",
			},
		}

		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request state doesn't require a build: DECLINED")
	}

	t.Log("Open")
	{
		pullRequest := PullRequestEventModel{
			PullRequestInfo: PullRequestInfoModel{
				ID:    1,
				Type:  "pullrequest",
				Title: "Title of pull request",
				State: "OPEN",
				SourceInfo: PullRequestBranchInfoModel{
					BranchInfo: BranchInfoModel{
						Name: "branch2",
					},
					CommitInfo: CommitInfoModel{
						CommitHash: "d3022fc0ca3d",
					},
					RepositoryInfo: RepositoryInfoModel{
						FullName: "foo/myrepo",
					},
				},
				DestinationInfo: PullRequestBranchInfoModel{
					BranchInfo: BranchInfoModel{
						Name: "master",
					},
					CommitInfo: CommitInfoModel{
						CommitHash: "ce5965ddd289",
					},
					RepositoryInfo: RepositoryInfoModel{
						FullName: "foo/myrepo",
					},
				},
			},
		}

		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "d3022fc0ca3d",
					CommitMessage:            "Title of pull request",
					Branch:                   "branch2",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(1),
					PullRequestRepositoryURL: "https://bitbucket.org/foo/myrepo.git",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - Title & Body")
	{
		pullRequest := PullRequestEventModel{
			PullRequestInfo: PullRequestInfoModel{
				ID:          1,
				Type:        "pullrequest",
				Title:       "Title of pull request",
				Description: "Description of pull request",
				State:       "OPEN",
				SourceInfo: PullRequestBranchInfoModel{
					BranchInfo: BranchInfoModel{
						Name: "branch2",
					},
					CommitInfo: CommitInfoModel{
						CommitHash: "d3022fc0ca3d",
					},
					RepositoryInfo: RepositoryInfoModel{
						FullName: "foo/myrepo",
					},
				},
				DestinationInfo: PullRequestBranchInfoModel{
					BranchInfo: BranchInfoModel{
						Name: "master",
					},
					CommitInfo: CommitInfoModel{
						CommitHash: "ce5965ddd289",
					},
					RepositoryInfo: RepositoryInfoModel{
						FullName: "foo/myrepo",
					},
				},
			},
		}

		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "d3022fc0ca3d",
					CommitMessage:            "Title of pull request\n\nDescription of pull request",
					Branch:                   "branch2",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(1),
					PullRequestRepositoryURL: "https://bitbucket.org/foo/myrepo.git",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_isAcceptEventType(t *testing.T) {
	t.Log("Accept")
	{
		for _, anAction := range []string{"repo:push",
			"pullrequest:created", "pullrequest:updated",
		} {
			t.Log(" * " + anAction)
			require.Equal(t, true, isAcceptEventType(anAction))
		}
	}

	t.Log("Don't accept")
	{
		for _, anAction := range []string{"",
			"a", "not-an-action",
			"pullrequest:approved", "pullrequest:unapproved", "pullrequest:fulfilled", "pullrequest:rejected",
			"pullrequest:comment_created", "pullrequest:comment_updated", "pull_request:comment_deleted",
		} {
			t.Log(" * " + anAction)
			require.Equal(t, false, isAcceptEventType(anAction))
		}
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Test with Sample Mercurial Code Push data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"repo:push"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"1"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleMercurialCodePushData)),
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
	t.Log("Test with Sample Tag Push data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"repo:push"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"1"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleTagPushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.2",
					CommitHash:    "966d0bfe79b80f97268c2f6bb45e65e79ef09b31",
					CommitMessage: "auto-test",
				},
			},
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.1",
					CommitHash:    "19934139a2cf799bbd0f5061ab02e4760902e93f",
					CommitMessage: "auto-test 2",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Test with Sample Pull Request data")
	{
		request := http.Request{
			Header: http.Header{
				"X-Event-Key":      {"pullrequest:created"},
				"Content-Type":     {"application/json"},
				"X-Attempt-Number": {"1"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePullRequestData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "6a3451888d91",
					CommitMessage:            "change",
					Branch:                   "feature/test",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(1),
					PullRequestRepositoryURL: "https://bitbucket.org/birmacher/prtest.git",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}
