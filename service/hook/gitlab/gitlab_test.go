package gitlab

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/bitrise-io/go-utils/pointers"
	"github.com/stretchr/testify/require"
)

const sampleCodePushData = `{
"object_kind": "push",
"ref": "refs/heads/develop",
"checkout_sha": "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
"user_username": "test_user",
"commits": [
	{
		"id": "29da60ce2c47a6696bc82f2e6ec4a075695eb7c3",
		"message": "first commit message"
	},
	{
		"id": "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
		"message": "second commit message"
	}
]
}`

const sampleMergeRequestData = `{
"object_kind": "merge_request",
"user": {
	"name": "Author Name",
	"username": "test_user"
},
"object_attributes": {
	"target_branch": "develop",
	"source_branch": "feature/gitlab-pr",
	"title": "PR test",
	"merge_status": "unchecked",
	"iid": 12,
	"description": "PR text body",
	"merge_error": null,
	"merge_commit_sha": null,
	"source": {
		"git_ssh_url": "git@gitlab.com:bitrise-io/bitrise-webhooks.git",
		"git_http_url": "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
		"namespace":"bitrise-io",
		"visibility_level": 20
	},
	"target": {
		"git_ssh_url": "git@gitlab.com:bitrise-io/bitrise-webhooks.git",
		"git_http_url": "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
		"namespace":"bitrise-team",
		"visibility_level": 20
	},
	"last_commit": {
		"id": "da966425f32973b6290dcff6a443103c7ff2a8cb"
	},
	"action": "update",
	"oldrev": "3c86b996d8014000a93f3c202fc0963e81e56c4c",
	"state": "opened"
}}`

const sampleForkMergeRequestData = `{
	"object_kind": "merge_request",
	"user": {
		"name": "Author Name",
		"username": "test_user"
	},
	"object_attributes": {
		"target_branch": "develop",
		"source_branch": "feature/gitlab-pr",
		"title": "PR test",
		"merge_status": "unchecked",
		"iid": 12,
		"description": "PR text body",
		"merge_error": null,
		"merge_commit_sha": null,
		"source": {
			"git_ssh_url": "git@gitlab.com:oss-contributor/fork-bitrise-webhooks.git",
			"git_http_url": "https://gitlab.com/oss-contributor/fork-bitrise-webhooks.git",
			"namespace":"oss-contributor",
			"visibility_level": 20
		},
		"target": {
			"git_ssh_url": "git@gitlab.com:bitrise-io/bitrise-webhooks.git",
			"git_http_url": "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
			"namespace":"bitrise-io",
			"visibility_level": 20
		},
		"last_commit": {
			"id": "da966425f32973b6290dcff6a443103c7ff2a8cb"
		},
		"action": "update",
		"oldrev": "3c86b996d8014000a93f3c202fc0963e81e56c4c",
		"state": "opened"
	}}`

func Test_detectContentTypeAndEventID(t *testing.T) {
	t.Log("Code Push event")
	{
		header := http.Header{
			"X-Gitlab-Event": {"Push Hook"},
			"Content-Type":   {"application/json"},
		}
		contentType, eventID, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "Push Hook", eventID)
	}

	t.Log("Tag Push event")
	{
		header := http.Header{
			"X-Gitlab-Event": {"Tag Push Hook"},
			"Content-Type":   {"application/json"},
		}
		contentType, eventID, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "Tag Push Hook", eventID)
	}

	t.Log("Merge Request event - should handle")
	{
		header := http.Header{
			"X-Gitlab-Event": {"Merge Request Hook"},
			"Content-Type":   {"application/json"},
		}
		contentType, glEvent, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "Merge Request Hook", glEvent)
	}

	t.Log("Unsupported event - will be handled (rejected) in Transform")
	{
		header := http.Header{
			"X-Gitlab-Event": {"Unsupported Hook"},
			"Content-Type":   {"application/json"},
		}
		contentType, eventID, err := detectContentTypeAndEventID(header)
		require.NoError(t, err)
		require.Equal(t, "application/json", contentType)
		require.Equal(t, "Unsupported Hook", eventID)
	}

	t.Log("Missing X-Gitlab-Event header")
	{
		header := http.Header{
			"Content-Type": {"application/json"},
		}
		contentType, eventID, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "No X-Gitlab-Event Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventID)
	}

	t.Log("Missing Content-Type")
	{
		header := http.Header{
			"X-Gitlab-Event": {"Push Hook"},
		}
		contentType, eventID, err := detectContentTypeAndEventID(header)
		require.EqualError(t, err, "No Content-Type Header found")
		require.Equal(t, "", contentType)
		require.Equal(t, "", eventID)
	}
}

func Test_transformCodePushEvent(t *testing.T) {
	t.Log("Do Transform - single commit")
	{
		codePush := CodePushEventModel{
			ObjectKind:   "push",
			Ref:          "refs/heads/master",
			CheckoutSHA:  "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
			UserUsername: "test_user",
			Commits: []CommitModel{
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
					Branch:        "master",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Do Transform - multiple commits - CheckoutSHA match should trigger the build")
	{
		codePush := CodePushEventModel{
			ObjectKind:   "push",
			Ref:          "refs/heads/master",
			CheckoutSHA:  "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
			UserUsername: "test_user",
			Commits: []CommitModel{
				{
					CommitHash:    "7782203aaf0daabbd245ec0370c751eac6a4eb55",
					CommitMessage: `switch to three component versions`,
				},
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
				},
				{
					CommitHash:    "ef77f9dba498f335e2e7078bdb52f9e11c214c58",
					CommitMessage: `get version : three component version`,
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
					Branch:        "master",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("No commit matches CheckoutSHA")
	{
		codePush := CodePushEventModel{
			ObjectKind:   "push",
			Ref:          "refs/heads/master",
			CheckoutSHA:  "checkout-sha",
			UserUsername: "test_user",
			Commits: []CommitModel{
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "The commit specified by 'checkout_sha' was not included in the 'commits' array - no match found")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a head ref")
	{
		codePush := CodePushEventModel{
			ObjectKind:   "push",
			Ref:          "refs/not/head",
			CheckoutSHA:  "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
			UserUsername: "test_user",
			Commits: []CommitModel{
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: `Response: omit the "failed_responses" array if empty`,
				},
			},
		}
		hookTransformResult := transformCodePushEvent(codePush)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Ref (refs/not/head) is not a head ref")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_transformTagPushEvent(t *testing.T) {
	t.Log("Do Transform")
	{
		tagPush := TagPushEventModel{
			ObjectKind:   "tag_push",
			Ref:          "refs/tags/v0.0.2",
			CheckoutSHA:  "7f29cdf31fdff43d7f31a279eec06c9f19ae0d6b",
			UserUsername: "test_user",
		}
		hookTransformResult := transformTagPushEvent(tagPush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        "v0.0.2",
					CommitHash: "7f29cdf31fdff43d7f31a279eec06c9f19ae0d6b",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("No CheckoutSHA (tag delete)")
	{
		tagPush := TagPushEventModel{
			ObjectKind:   "tag_push",
			Ref:          "refs/tags/v0.0.2",
			CheckoutSHA:  "",
			UserUsername: "test_user",
		}
		hookTransformResult := transformTagPushEvent(tagPush)
		require.EqualError(t, hookTransformResult.Error, "This is a Tag Deleted event, no build is required")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a tags ref")
	{
		tagPush := TagPushEventModel{
			ObjectKind:   "tag_push",
			Ref:          "refs/not/a/tag",
			CheckoutSHA:  "7f29cdf31fdff43d7f31a279eec06c9f19ae0d6b",
			UserUsername: "test_user",
		}
		hookTransformResult := transformTagPushEvent(tagPush)
		require.EqualError(t, hookTransformResult.Error, "Ref (refs/not/a/tag) is not a tags ref")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not a tag_push object")
	{
		tagPush := TagPushEventModel{
			ObjectKind:   "not-a-tag_push",
			Ref:          "refs/tags/v0.0.2",
			CheckoutSHA:  "7f29cdf31fdff43d7f31a279eec06c9f19ae0d6b",
			UserUsername: "test_user",
		}
		hookTransformResult := transformTagPushEvent(tagPush)
		require.EqualError(t, hookTransformResult.Error, "Not a Tag Push object: not-a-tag_push")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_transformMergeRequestEvent(t *testing.T) {
	t.Log("Unsupported Merge Request kind")
	{
		mergeRequest := MergeRequestEventModel{
			ObjectKind: "labeled",
		}
		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Not a Merge Request object")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Empty Merge Request state")
	{
		mergeRequest := MergeRequestEventModel{
			ObjectKind:       "merge_request",
			ObjectAttributes: ObjectAttributesInfoModel{},
		}
		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "No Merge Request state specified")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Already Merged")
	{
		mergeRequest := MergeRequestEventModel{
			ObjectKind: "merge_request",
			ObjectAttributes: ObjectAttributesInfoModel{
				State:          "opened",
				MergeCommitSHA: "asd123",
			},
		}
		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Merge Request already merged")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Merge error")
	{
		mergeRequest := MergeRequestEventModel{
			ObjectKind: "merge_request",
			ObjectAttributes: ObjectAttributesInfoModel{
				State:      "opened",
				Action:     "update",
				Oldrev:     "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				MergeError: "Some merge error",
			},
		}
		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Merge Request is not mergeable")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not mergeable")
	{
		mergeRequest := MergeRequestEventModel{
			ObjectKind: "merge_request",
			ObjectAttributes: ObjectAttributesInfoModel{
				State:       "opened",
				Action:      "update",
				Oldrev:      "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				MergeStatus: "cannot_be_merged",
			},
		}
		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Merge Request is not mergeable")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Irrelevant action")
	{
		mergeRequest := MergeRequestEventModel{
			ObjectKind: "merge_request",
			ObjectAttributes: ObjectAttributesInfoModel{
				State:  "opened",
				Action: "approved",
			},
		}
		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Merge Request action doesn't require a build: approved")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Irrelevant update")
	{
		mergeRequest := MergeRequestEventModel{
			ObjectKind: "merge_request",
			ObjectAttributes: ObjectAttributesInfoModel{
				State:  "opened",
				Action: "update",
			},
		}
		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.True(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Merge Request action doesn't require a build: update")
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Not yet merged")
	{
		mergeRequest := MergeRequestEventModel{
			User: UserModel{
				Username: "test_user",
			},
			ObjectKind: "merge_request",
			ObjectAttributes: ObjectAttributesInfoModel{
				ID:     12,
				Title:  "PR test",
				State:  "opened",
				Action: "open",
				Source: BranchInfoModel{
					VisibilityLevel: 20,
					GitSSHURL:       "git@gitlab.com:bitrise-io/bitrise-webhooks.git",
					GitHTTPURL:      "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
				},
				SourceBranch: "feature/gitlab-pr",
				Target: BranchInfoModel{
					VisibilityLevel: 20,
					GitSSHURL:       "git@gitlab.com:bitrise-io/bitrise-webhooks.git",
					GitHTTPURL:      "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
				},
				TargetBranch: "master",
				LastCommit: LastCommitInfoModel{
					SHA: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				},
			},
		}

		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:            "PR test",
					Branch:                   "feature/gitlab-pr",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(12),
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "merge-requests/12/merge",
					PullRequestHeadBranch:    "merge-requests/12/head",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Pull Request - Title & Body")
	{
		mergeRequest := MergeRequestEventModel{
			User: UserModel{
				Username: "test_user",
			},
			ObjectKind: "merge_request",
			ObjectAttributes: ObjectAttributesInfoModel{
				ID:          12,
				Title:       "PR test",
				Description: "PR test body",
				State:       "opened",
				Action:      "open",
				Source: BranchInfoModel{
					VisibilityLevel: 20,
					GitSSHURL:       "git@gitlab.com:bitrise-io/bitrise-webhooks.git",
					GitHTTPURL:      "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
				},
				SourceBranch: "feature/gitlab-pr",
				Target: BranchInfoModel{
					VisibilityLevel: 20,
					GitSSHURL:       "git@gitlab.com:bitrise-io/bitrise-webhooks.git",
					GitHTTPURL:      "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
				},
				TargetBranch: "master",
				LastCommit: LastCommitInfoModel{
					SHA: "83b86e5f286f546dc5a4a58db66ceef44460c85e",
				},
			},
		}

		hookTransformResult := transformMergeRequestEvent(mergeRequest)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "83b86e5f286f546dc5a4a58db66ceef44460c85e",
					CommitMessage:            "PR test\n\nPR test body",
					Branch:                   "feature/gitlab-pr",
					BranchDest:               "master",
					PullRequestID:            pointers.NewIntPtr(12),
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "merge-requests/12/merge",
					PullRequestHeadBranch:    "merge-requests/12/head",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}
}

func Test_isAcceptEventType(t *testing.T) {
	t.Log("Accept")
	{
		for _, anEvent := range []string{
			"Push Hook", "Merge Request Hook", "Tag Push Hook",
		} {
			t.Log(" * " + anEvent)
			require.Equal(t, true, isAcceptEventType(anEvent))
		}
	}

	t.Log("Don't accept")
	{
		for _, anEvent := range []string{"",
			"a", "not-an-action",
			"Issue Hook", "Note Hook", "Wiki Page Hook"} {
			t.Log(" * " + anEvent)
			require.Equal(t, false, isAcceptEventType(anEvent))
		}
	}
}

func Test_getRepositoryURL(t *testing.T) {
	t.Log("Visibility == 0")
	{
		branchInfoModel := BranchInfoModel{
			VisibilityLevel: 0,
			GitSSHURL:       "git@gitlab.com:test/test-repo.git",
			GitHTTPURL:      "https://gitlab.com/test/test-repo.git",
		}

		t.Log(fmt.Sprintf(" Visibility: %d", branchInfoModel.VisibilityLevel))
		require.Equal(t, "git@gitlab.com:test/test-repo.git", branchInfoModel.getRepositoryURL())
	}

	t.Log("Visibility == 10")
	{
		branchInfoModel := BranchInfoModel{
			VisibilityLevel: 10,
			GitSSHURL:       "git@gitlab.com:test/test-repo.git",
			GitHTTPURL:      "https://gitlab.com/test/test-repo.git",
		}

		t.Log(fmt.Sprintf(" Visibility: %d", branchInfoModel.VisibilityLevel))
		require.Equal(t, "git@gitlab.com:test/test-repo.git", branchInfoModel.getRepositoryURL())
	}

	t.Log("Visibility == 20")
	{
		branchInfoModel := BranchInfoModel{
			VisibilityLevel: 20,
			GitSSHURL:       "git@gitlab.com:test/test-repo.git",
			GitHTTPURL:      "https://gitlab.com/test/test-repo.git",
		}

		t.Log(fmt.Sprintf(" Visibility: %d", branchInfoModel.VisibilityLevel))
		require.Equal(t, "https://gitlab.com/test/test-repo.git", branchInfoModel.getRepositoryURL())
	}
}

func Test_HookProvider_TransformRequest(t *testing.T) {
	provider := HookProvider{}

	t.Log("Code Push - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gitlab-Event": {"Push Hook"},
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
					CommitHash:    "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
					CommitMessage: "second commit message",
					Branch:        "develop",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Push: Tag (create)")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gitlab-Event": {"Tag Push Hook"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(`{
  "object_kind": "tag_push",
  "ref": "refs/tags/v0.0.2",
  "checkout_sha": "7f29cdf31fdff43d7f31a279eec06c9f19ae0d6b",
  "user_username": "test_user"
}`)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					Tag:        "v0.0.2",
					CommitHash: "7f29cdf31fdff43d7f31a279eec06c9f19ae0d6b",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Push: Tag Delete")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gitlab-Event": {"Tag Push Hook"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(`{
  "object_kind": "tag_push",
  "ref": "refs/tags/v0.0.2",
  "checkout_sha": null,
  "user_username": "test_user"
}`)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.EqualError(t, hookTransformResult.Error, "This is a Tag Deleted event, no build is required")
		require.True(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Merge Request - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gitlab-Event": {"Merge Request Hook"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleMergeRequestData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "da966425f32973b6290dcff6a443103c7ff2a8cb",
					CommitMessage:            "PR test\n\nPR text body",
					Branch:                   "feature/gitlab-pr",
					BranchRepoOwner:          "bitrise-io",
					BranchDest:               "develop",
					BranchDestRepoOwner:      "bitrise-team",
					PullRequestID:            pointers.NewIntPtr(12),
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestAuthor:        "Author Name",
					PullRequestMergeBranch:   "merge-requests/12/merge",
					PullRequestHeadBranch:    "merge-requests/12/head",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Fork Merge Request - should be handled")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gitlab-Event": {"Merge Request Hook"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleForkMergeRequestData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:               "da966425f32973b6290dcff6a443103c7ff2a8cb",
					CommitMessage:            "PR test\n\nPR text body",
					Branch:                   "feature/gitlab-pr",
					BranchRepoOwner:          "oss-contributor",
					BranchDest:               "develop",
					BranchDestRepoOwner:      "bitrise-io",
					PullRequestID:            pointers.NewIntPtr(12),
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/oss-contributor/fork-bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/oss-contributor/fork-bitrise-webhooks.git",
					PullRequestAuthor:        "Author Name",
					PullRequestMergeBranch:   "merge-requests/12/merge",
					PullRequestHeadBranch:    "merge-requests/12/head",
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Unsuported Content-Type")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gitlab-Event": {"Push Hook"},
				"Content-Type":   {"not/supported"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Content-Type is not supported: not/supported")
	}

	t.Log("Unsupported event type - should error")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gitlab-Event": {"Unsupported Hook"},
				"Content-Type":   {"application/json"},
			},
			Body: ioutil.NopCloser(strings.NewReader(sampleCodePushData)),
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Unsupported Webhook event: Unsupported Hook")
	}

	t.Log("No Request Body")
	{
		request := http.Request{
			Header: http.Header{
				"X-Gitlab-Event": {"Push Hook"},
				"Content-Type":   {"application/json"},
			},
		}
		hookTransformResult := provider.TransformRequest(&request)
		require.False(t, hookTransformResult.ShouldSkip)
		require.EqualError(t, hookTransformResult.Error, "Failed to read content of request body: no or empty request body")
	}
}
