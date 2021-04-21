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
  },
  "commits": [
    {
      "added": [
        "added/file/path"
      ],
      "removed": [
        "removed/file/path"
      ],
      "modified": [
        "modified/file/path"
      ]
    }
  ],
  "repository": {
	"private": false,
	"ssh_url": "git@github.com:bitrise-team/bitrise-webhooks.git",
	"clone_url": "https://github.com/bitrise-team/bitrise-webhooks.git",
	"owner": {
		"login": "bitrise-team"
	}
  }
}`

	sampleTagPushData = `{
  "ref": "refs/tags/v0.0.2",
  "deleted": false,
  "head_commit": {
    "distinct": true,
    "id": "2e197ebd2330183ae11338151cf3a75db0c23c92",
    "message": "generalize Push Event (previously Code Push)\n\nwe'll handle the Tag Push too, so related codes are changed to reflect this (removed code from CodePush - e.g. CodePushEventModel -> PushEventModel)"
  },
  "commits": [
    {
      "added": [
        "added/file/path"
      ],
      "removed": [
        "removed/file/path"
      ],
      "modified": [
        "modified/file/path"
      ]
    }
  ],
  "repository": {
	"private": false,
	"ssh_url": "git@github.com:bitrise-team/bitrise-webhooks.git",
	"clone_url": "https://github.com/bitrise-team/bitrise-webhooks.git",
	"owner": {
		"login": "bitrise-team"
	}
  }
}`

	samplePullRequestData = `{
	"action": "opened",
	"number": 12,
	"pull_request": {
		"diff_url": "https://github.com/bitrise-io/bitrise-webhooks/pull/1.diff",
		"head": {
			"ref": "feature/github-pr",
			"sha": "83b86e5f286f546dc5a4a58db66ceef44460c85e",
			"repo": {
				"private": false,
				"ssh_url": "git@github.com:bitrise-team/bitrise-webhooks.git",
				"clone_url": "https://github.com/bitrise-team/bitrise-webhooks.git",
				"owner": {
					"login": "bitrise-team"
				}
			}
		},
		"base": {
			"ref": "master",
			"sha": "3c86b996d8014000a93f3c202fc0963e81e56c4c",
			"repo": {
				"private": false,
				"ssh_url": "git@github.com:bitrise-io/bitrise-webhooks.git",
				"clone_url": "https://github.com/bitrise-io/bitrise-webhooks.git",
				"owner": {
					"login": "bitrise-io"
				}
			}
		},
		"title": "PR test",
		"body": "PR text body",
		"merged": false,
		"mergeable": true,
		"user": {
			"login": "Author Name"
		}
	}
}`

	samplePullRequestEditedData = `{
  "action": "edited",
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
      "ref": "develop",
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
  },
  "changes": {
    "body": {
      "from": "this is the PR body"
    },
    "title": {
      "from": "auto-test - title change - without SKIP CI"
    },
    "base": {
      "ref": {
        "from": "master"
      },
      "sha": {
        "from": "bac0e53691fd6fbc5e8c4f00144bf61069b80087"
      }
    }
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
		require.EqualError(t, err, "No X-Github-Event Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", ghEvent)
	}

	t.Log("Missing Content-Type")
	{
		header := http.Header{
			"X-Github-Event": {"push"},
		}
		contentType, ghEvent, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "No Content-Type Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", ghEvent)
	}
}

func Test_transformPushEvent(t *testing.T) {
	t.Log("Do Transform - Code Push")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Do Transform - Tag Push")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "2e197ebd2330183ae11338151cf3a75db0c23c92",
				CommitMessage: "generalize Push Event (previously Code Push)",
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.2",
					CommitHash:    "2e197ebd2330183ae11338151cf3a75db0c23c92",
					CommitMessage: "generalize Push Event (previously Code Push)",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not Distinct Head Commit - should still trigger a build (e.g. this can happen if you rebase-merge a PR, without creating a merge commit)")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      false,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
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
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Tag - Not Distinct Head Commit - should still trigger a build")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			HeadCommit: CommitModel{
				Distinct:      false,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:           "v0.0.2",
					CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage: "re-structuring Hook Providers, with added tests",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Missing Commit Hash")
	{
		codePush := PushEventModel{
			Ref: "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "Missing commit hash")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Missing Commit Hash - Tag")
	{
		tagPush := PushEventModel{
			Ref: "refs/tags/v0.0.2",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.EqualError(t, hookTransformResult.Error, "Missing commit hash")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("This is a 'deleted' event")
	{
		codePush := PushEventModel{
			Deleted: true,
			Ref:     "refs/heads/master",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "This is a 'Deleted' event, no build can be started")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("This is a 'deleted' event - Tag")
	{
		tagPush := PushEventModel{
			Deleted: true,
			Ref:     "refs/tags/v0.0.2",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformPushEvent(tagPush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "This is a 'Deleted' event, no build can be started")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a head nor a tag ref")
	{
		codePush := PushEventModel{
			Ref: "refs/not/head",
			HeadCommit: CommitModel{
				Distinct:      true,
				CommitHash:    "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				CommitMessage: "re-structuring Hook Providers, with added tests",
			},
		}
		hookTransformResult := transformPushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Ref (refs/not/head) is not a head nor a tag ref")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
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
					BaseRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
					PullRequestHeadBranch:    "pull/12/head",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
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
					BaseRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
					PullRequestHeadBranch:    "pull/12/head",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
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
					BaseRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
					PullRequestHeadBranch:    "pull/12/head",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - edited - only title change - no skip ci change - no build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "edited",
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
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Changes: PullRequestChangesInfoModel{
				Title: PullRequestChangeFromItemModel{
					From: "previous title",
				},
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.EqualError(t, hookTransformResult.Error, "Pull Request edit doesn't require a build: only title and/or description was changed, and previous one was not skipped")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel(nil), hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - edited - only title changed - BUT the previous title included a skip CI pattern - should build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "edited",
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
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Changes: PullRequestChangesInfoModel{
				Title: PullRequestChangeFromItemModel{
					From: "previous title with [skip ci] in it",
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
					BranchDest:               "develop",
					PullRequestID:            pointers.NewIntPtr(12),
					PullRequestRepositoryURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
					PullRequestHeadBranch:    "pull/12/head",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - edited - only body/description change - no skip ci in previous - no build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "edited",
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
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Changes: PullRequestChangesInfoModel{
				Body: PullRequestChangeFromItemModel{
					From: "previous body",
				},
			},
		}
		hookTransformResult := transformPullRequestEvent(pullRequest)
		require.EqualError(t, hookTransformResult.Error, "Pull Request edit doesn't require a build: only title and/or description was changed, and previous one was not skipped")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel(nil), hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - edited - only body/description change - BUT the previous title included a skip CI pattern - should build")
	{
		pullRequest := PullRequestEventModel{
			Action:        "edited",
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
					Ref:        "develop",
					CommitHash: "3c86b996d8014000a93f3c202fc0963e81e56c4c",
					Repo: RepoInfoModel{
						Private:  false,
						SSHURL:   "git@github.com:bitrise-io/bitrise-webhooks.git",
						CloneURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					},
				},
			},
			Changes: PullRequestChangesInfoModel{
				Body: PullRequestChangeFromItemModel{
					From: "previous body with [skip ci] in it",
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
					BranchDest:               "develop",
					PullRequestID:            pointers.NewIntPtr(12),
					PullRequestRepositoryURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
					PullRequestHeadBranch:    "pull/12/head",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
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

	t.Log("Push Event - should not be skipped")
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

	t.Log("Pull Request Event - should not be skipped")
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
					PushCommitPaths: []bitriseapi.CommitPaths{
						bitriseapi.CommitPaths{
							Added:    []string{"added/file/path"},
							Removed:  []string{"removed/file/path"},
							Modified: []string{"modified/file/path"},
						},
					},
					BaseRepositoryURL: "https://github.com/bitrise-team/bitrise-webhooks.git",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Tag Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"push"},
				"Content-Type":   {"application/json"},
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
					CommitHash:    "2e197ebd2330183ae11338151cf3a75db0c23c92",
					CommitMessage: "generalize Push Event (previously Code Push)\n\nwe'll handle the Tag Push too, so related codes are changed to reflect this (removed code from CodePush - e.g. CodePushEventModel -> PushEventModel)",
					PushCommitPaths: []bitriseapi.CommitPaths{
						bitriseapi.CommitPaths{
							Added:    []string{"added/file/path"},
							Removed:  []string{"removed/file/path"},
							Modified: []string{"modified/file/path"},
						},
					},
					BaseRepositoryURL: "https://github.com/bitrise-team/bitrise-webhooks.git",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
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
					DiffURL:                  "https://github.com/bitrise-io/bitrise-webhooks/pull/1.diff",
					CommitHash:               "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:            "PR test\n\nPR text body",
					Branch:                   "feature/github-pr",
					BranchRepoOwner:          "bitrise-team",
					BranchDest:               "master",
					BranchDestRepoOwner:      "bitrise-io",
					PullRequestID:            pointers.NewIntPtr(12),
					PullRequestRepositoryURL: "https://github.com/bitrise-team/bitrise-webhooks.git",
					BaseRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://github.com/bitrise-team/bitrise-webhooks.git",
					PullRequestAuthor:        "Author Name",
					PullRequestMergeBranch:   "pull/12/merge",
					PullRequestHeadBranch:    "pull/12/head",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request :: edited - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Github-Event": {"pull_request"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(samplePullRequestEditedData)),
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
					BranchDest:               "develop",
					PullRequestID:            pointers.NewIntPtr(12),
					PullRequestRepositoryURL: "https://github.com/bitrise-io/bitrise-webhooks.git",
					BaseRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://github.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "pull/12/merge",
					PullRequestHeadBranch:    "pull/12/head",
				},
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, false, hookTransformResult.DontWaitForTriggerResponse)
	}
}
