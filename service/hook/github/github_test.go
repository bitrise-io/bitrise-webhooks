package github

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
  "ref": "refs/heads/master",
  "deleted": false,
  "head_commit": {
    "distinct": true,
    "id": "83b86e5f286f546dc5a4a58db66ceef44460c85e",
    "message": "re-structuring Hook Providers, with added tests"
  }
}`

	samplePullRequestData = `{
  "action": "opened",
  "number": 12,
  "pull_request": {
    "head": {
      "ref": "feature/github-pr",
      "sha": "83b86e5f286f546dc5a4a58db66ceef44460c85e",
      "repo" : {
        "private": false,
        "ssh_url": "git@github.com:bitrise-io/bitrise-webhooks.git",
        "clone_url": "https://github.com/bitrise-io/bitrise-webhooks.git"
      }
    },
    "base": {
      "ref": "master",
      "sha": "3c86b996d8014000a93f3c202fc0963e81e56c4c",
      "repo" : {
        "private": false,
        "ssh_url": "git@github.com:bitrise-io/bitrise-webhooks.git",
        "clone_url": "https://github.com/bitrise-io/bitrise-webhooks.git"
      }
    },
    "title": "PR test",
    "body": "PR text body",
    "merged": false,
    "mergeable": true
  }
}`
)

func Test_detectContentTypeAndEventID(t *testing.T) {
	t.Log("Push event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "push", ghEvent)
	}

	t.Log("Pull Request event - should handle")
	{
		header := http.Header{
			"X-Github-Event": {"pull_request"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "pull_request", ghEvent)
	}

	t.Log("Ping event")
	{
		header := http.Header{
			"X-Github-Event": {"ping"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "ping", ghEvent)
	}

	t.Log("Unsupported GH event - will be handled in Transform")
	{
		header := http.Header{
			"X-Github-Event": {"label"},
			"Content-Type":   {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "label", ghEvent)
	}

	t.Log("Missing X-Github-Event header")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "Issue with X-Github-Event Header: No value found in HEADER for the key: X-Github-Event")
		require.Equal(t, "", contentType)
		require.Equal(t, "", ghEvent)
	}

	t.Log("Missing Content-Type")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "Issue with Content-Type Header: No value found in HEADER for the key: Content-Type")
		require.Equal(t, "", contentType)
		require.Equal(t, "", ghEvent)
	}
}

func Test_transformCodePushEvent(t *testing.T) {
	t.Log("Do Transform")
	{
		codePush := CodePushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Not Distinct Head Commit - should trigger a build")
	{
		codePush := CodePushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      false,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Missing Commit Hash")
	{
		codePush := CodePushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "Missing commit hash")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("This is a 'deleted' event")
	{
		codePush := CodePushEventModel{
			Deleted: true,
			Ref:     "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "This is a 'Deleted' event, no build can be started")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Not a head ref")
	{
		codePush := CodePushEventModel{
			Ref: "refs/not/head",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Ref (refs/not/head) is not a head ref")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
	}
}

func Test_transformPullRequestEvent(t *testing.T) {
	t.Log("Unsupported Pull Request action")
	{
		pullRequest := PullRequestEventModel{
			Action: "labeled",
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request action doesn't require a build: labeled")
	}

	t.Log("Empty Pull Request action")
	{
		pullRequest := PullRequestEventModel{}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "No Pull Request action specified")
	}

	t.Log("Already Merged")
	{
		pullRequest := PullRequestEventModel{
			Action: "opened",
			PullRequestInfo: PullRequestInfoModel{
				Merged: true,
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request already merged")
	}

	t.Log("Not Mergeable")
	{
		pullRequest := PullRequestEventModel{
			Action: "reopened",
			PullRequestInfo: PullRequestInfoModel{
				Merged:    false,
				Mergeable: pointers.NewBoolPtr(false),
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Pull Request is not mergeable")
	}

	t.Log("Mergeable: not yet decided")
	{
		pullRequest := PullRequestEventModel{
			Action:        "synchronize",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Merged:    false,
				Mergeable: nil,
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.False(t, hookTransformResult.ShouldSkip)
		require.NoError(t, hookTransformResult.Error)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:            "PR test",
					Branch:                   "feature/github-pr",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(12),
					PullRequestRepositoryURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Mergeable: true")
	{
		pullRequest := PullRequestEventModel{
			Action:        "synchronize",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Merged:    false,
				Mergeable: pointers.NewBoolPtr(true),
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
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
					CommitHash:               "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:            "PR test",
					Branch:                   "feature/github-pr",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(12),
					PullRequestRepositoryURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Pull Request - Title & Body")
	{
		pullRequest := PullRequestEventModel{
			Action:        "opened",
			PullRequestID: 12,
			PullRequestInfo: PullRequestInfoModel{
				Title:     "PR test",
				Body:      "PR text body",
				Merged:    false,
				Mergeable: pointers.NewBoolPtr(true),
				HeadBranchInfo: BranchInfoModel{
					Ref:        "feature/github-pr",
					CommitHash: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
				BaseBranchInfo: BranchInfoModel{
					Ref:        "master",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
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
					CommitHash:               "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:            "PR test\n\nPR text body",
					Branch:                   "feature/github-pr",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(12),
					PullRequestRepositoryURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}
}

func Test_isAcceptPullRequestAction(t *testing.T) {
	t.Log("Accept")
	{
		for _, anAction := range []string{"opened", "reopened", "synchronize", "edited"} {
			t.Log(" * " + anAction)
			require.Equal(t, true, isAcceptPullRequestAction(anAction))
		}
	}

	t.Log("Don't accept")
	{
		for _, anAction := range []string{"",
			"a", "not-an-action",
			"assigned", "unassigned", "labeled", "unlabeled", "closed"} {
			t.Log(" * " + anAction)
			require.Equal(t, false, isAcceptPullRequestAction(anAction))
		}
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Ping - should be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"ping"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Ping event received")
	}

	t.Log("Unsuported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"not/supported"},
				"X-Github-Event": {"ping"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("Unsupported event type - should error")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"label"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Unsupported GitHub Webhook event: label")
	}

	t.Log("Code Push - should not be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"push"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Pull Request - should not be skipped")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"pull_request"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"X-Github-Event": {"push"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}

	t.Log("Code Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"push"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
					Branch:        "master",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}

	t.Log("Pull Request - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"pull_request"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePullRequestData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:            "PR test\n\nPR text body",
					Branch:                   "feature/github-pr",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(12),
					PullRequestRepositoryURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
	}
}
