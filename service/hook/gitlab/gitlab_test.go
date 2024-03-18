package gitlab

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const sampleCodePushData = `{
"object_kind": "push",
"ref": "refs/heads/develop",
"checkout_sha": "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
"user_username": "test_user",
"commits": [
	{
		"id": "29da60ce2c47a6696bc82f2e6ec4a075695eb7c3",
		"message": "first commit message",
      "added": ["README.MD"],
      "modified": ["app/controller/application.rb"],
      "removed": []
	},
	{
		"id": "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
		"message": "second commit message",
      "added": ["CHANGELOG"],
      "modified": ["app/controller/application.rb"],
      "removed": []
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
		"merge_status": "can_be_merged",
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

const sampleMergeRequestLabelAddedData = `{
  "object_kind": "merge_request",
  "event_type": "merge_request",
  "user": {
    "id": 20498345,
    "name": "Test User",
    "username": "test-user",
    "avatar_url": "https://secure.gravatar.com/avatar/1c779cb21fd42b608b40f7c2757aa640e3e9e05e166dce9c98c3c7ae368d8d27?s=80&d=identicon",
    "email": "[REDACTED]"
  },
  "project": {
    "id": 55857800,
    "name": "webhook-test",
    "description": null,
    "web_url": "https://gitlab.com/test.user/webhook-test",
    "avatar_url": null,
    "git_ssh_url": "git@gitlab.com:test.user/webhook-test.git",
    "git_http_url": "https://gitlab.com/test.user/webhook-test.git",
    "namespace": "test.user",
    "visibility_level": 0,
    "path_with_namespace": "test.user/webhook-test",
    "default_branch": "main",
    "ci_config_path": "",
    "homepage": "https://gitlab.com/test.user/webhook-test",
    "url": "git@gitlab.com:test.user/webhook-test.git",
    "ssh_url": "git@gitlab.com:test.user/webhook-test.git",
    "http_url": "https://gitlab.com/test.user/webhook-test.git"
  },
  "object_attributes": {
    "assignee_id": null,
    "author_id": 20498345,
    "created_at": "2024-03-14 15:33:21 UTC",
    "description": "Edited description of pull request",
    "draft": false,
    "head_pipeline_id": null,
    "id": 288638999,
    "iid": 1,
    "last_edited_at": "2024-03-14 15:34:41 UTC",
    "last_edited_by_id": 20498345,
    "merge_commit_sha": null,
    "merge_error": null,
    "merge_params": {
      "force_remove_source_branch": "1"
    },
    "merge_status": "can_be_merged",
    "merge_user_id": null,
    "merge_when_pipeline_succeeds": false,
    "milestone_id": null,
    "source_branch": "brencs",
    "source_project_id": 55857800,
    "state_id": 1,
    "target_branch": "main",
    "target_project_id": 55857800,
    "time_estimate": 0,
    "title": "Test PR",
    "updated_at": "2024-03-14 15:36:49 UTC",
    "updated_by_id": 20498345,
    "prepared_at": "2024-03-14 15:33:23 UTC",
    "url": "https://gitlab.com/test.user/webhook-test/-/merge_requests/1",
    "source": {
      "id": 55857800,
      "name": "webhook-test",
      "description": null,
      "web_url": "https://gitlab.com/test.user/webhook-test",
      "avatar_url": null,
      "git_ssh_url": "git@gitlab.com:test.user/webhook-test.git",
      "git_http_url": "https://gitlab.com/test.user/webhook-test.git",
      "namespace": "test.user",
      "visibility_level": 0,
      "path_with_namespace": "test.user/webhook-test",
      "default_branch": "main",
      "ci_config_path": "",
      "homepage": "https://gitlab.com/test.user/webhook-test",
      "url": "git@gitlab.com:test.user/webhook-test.git",
      "ssh_url": "git@gitlab.com:test.user/webhook-test.git",
      "http_url": "https://gitlab.com/test.user/webhook-test.git"
    },
    "target": {
      "id": 55857800,
      "name": "webhook-test",
      "description": null,
      "web_url": "https://gitlab.com/test.user/webhook-test",
      "avatar_url": null,
      "git_ssh_url": "git@gitlab.com:test.user/webhook-test.git",
      "git_http_url": "https://gitlab.com/test.user/webhook-test.git",
      "namespace": "test.user",
      "visibility_level": 0,
      "path_with_namespace": "test.user/webhook-test",
      "default_branch": "main",
      "ci_config_path": "",
      "homepage": "https://gitlab.com/test.user/webhook-test",
      "url": "git@gitlab.com:test.user/webhook-test.git",
      "ssh_url": "git@gitlab.com:test.user/webhook-test.git",
      "http_url": "https://gitlab.com/test.user/webhook-test.git"
    },
    "last_commit": {
      "id": "5240ea6a9194b7f5cf53d25926984f0b6c1b5ac4",
      "message": "commit\n",
      "title": "commit",
      "timestamp": "2024-03-14T16:31:05+01:00",
      "url": "https://gitlab.com/test.user/webhook-test/-/commit/5240ea6a9194b7f5cf53d25926984f0b6c1b5ac4",
      "author": {
        "name": "Test User",
        "email": "[REDACTED]"
      }
    },
    "work_in_progress": false,
    "total_time_spent": 0,
    "time_change": 0,
    "human_total_time_spent": null,
    "human_time_change": null,
    "human_time_estimate": null,
    "assignee_ids": [

    ],
    "reviewer_ids": [

    ],
    "labels": [
      {
        "id": 34921318,
        "title": "blue",
        "color": "#6699cc",
        "project_id": 55857800,
        "created_at": "2024-03-14 15:36:47 UTC",
        "updated_at": "2024-03-14 15:36:47 UTC",
        "template": false,
        "description": null,
        "type": "ProjectLabel",
        "group_id": null,
        "lock_on_merge": false
      },
      {
        "id": 34921284,
        "title": "green",
        "color": "#009966",
        "project_id": 55857800,
        "created_at": "2024-03-14 15:33:11 UTC",
        "updated_at": "2024-03-14 15:33:11 UTC",
        "template": false,
        "description": null,
        "type": "ProjectLabel",
        "group_id": null,
        "lock_on_merge": false
      },
      {
        "id": 34921282,
        "title": "red",
        "color": "#dc143c",
        "project_id": 55857800,
        "created_at": "2024-03-14 15:33:06 UTC",
        "updated_at": "2024-03-14 15:33:06 UTC",
        "template": false,
        "description": null,
        "type": "ProjectLabel",
        "group_id": null,
        "lock_on_merge": false
      }
    ],
    "state": "opened",
    "blocking_discussions_resolved": true,
    "first_contribution": true,
    "detailed_merge_status": "mergeable",
    "action": "update"
  },
  "labels": [
    {
      "id": 34921318,
      "title": "blue",
      "color": "#6699cc",
      "project_id": 55857800,
      "created_at": "2024-03-14 15:36:47 UTC",
      "updated_at": "2024-03-14 15:36:47 UTC",
      "template": false,
      "description": null,
      "type": "ProjectLabel",
      "group_id": null,
      "lock_on_merge": false
    },
    {
      "id": 34921284,
      "title": "green",
      "color": "#009966",
      "project_id": 55857800,
      "created_at": "2024-03-14 15:33:11 UTC",
      "updated_at": "2024-03-14 15:33:11 UTC",
      "template": false,
      "description": null,
      "type": "ProjectLabel",
      "group_id": null,
      "lock_on_merge": false
    },
    {
      "id": 34921282,
      "title": "red",
      "color": "#dc143c",
      "project_id": 55857800,
      "created_at": "2024-03-14 15:33:06 UTC",
      "updated_at": "2024-03-14 15:33:06 UTC",
      "template": false,
      "description": null,
      "type": "ProjectLabel",
      "group_id": null,
      "lock_on_merge": false
    }
  ],
  "changes": {
    "labels": {
      "previous": [
        {
          "id": 34921284,
          "title": "green",
          "color": "#009966",
          "project_id": 55857800,
          "created_at": "2024-03-14 15:33:11 UTC",
          "updated_at": "2024-03-14 15:33:11 UTC",
          "template": false,
          "description": null,
          "type": "ProjectLabel",
          "group_id": null,
          "lock_on_merge": false
        },
        {
          "id": 34921282,
          "title": "red",
          "color": "#dc143c",
          "project_id": 55857800,
          "created_at": "2024-03-14 15:33:06 UTC",
          "updated_at": "2024-03-14 15:33:06 UTC",
          "template": false,
          "description": null,
          "type": "ProjectLabel",
          "group_id": null,
          "lock_on_merge": false
        }
      ],
      "current": [
        {
          "id": 34921318,
          "title": "blue",
          "color": "#6699cc",
          "project_id": 55857800,
          "created_at": "2024-03-14 15:36:47 UTC",
          "updated_at": "2024-03-14 15:36:47 UTC",
          "template": false,
          "description": null,
          "type": "ProjectLabel",
          "group_id": null,
          "lock_on_merge": false
        },
        {
          "id": 34921284,
          "title": "green",
          "color": "#009966",
          "project_id": 55857800,
          "created_at": "2024-03-14 15:33:11 UTC",
          "updated_at": "2024-03-14 15:33:11 UTC",
          "template": false,
          "description": null,
          "type": "ProjectLabel",
          "group_id": null,
          "lock_on_merge": false
        },
        {
          "id": 34921282,
          "title": "red",
          "color": "#dc143c",
          "project_id": 55857800,
          "created_at": "2024-03-14 15:33:06 UTC",
          "updated_at": "2024-03-14 15:33:06 UTC",
          "template": false,
          "description": null,
          "type": "ProjectLabel",
          "group_id": null,
          "lock_on_merge": false
        }
      ]
    }
  },
  "repository": {
    "name": "webhook-test",
    "url": "git@gitlab.com:test.user/webhook-test.git",
    "description": null,
    "homepage": "https://gitlab.com/test.user/webhook-test"
  }
}`

var intTwelve = 12

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
		hookTransformResult := NewDefaultHookProvider(zap.NewNop()).transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:      "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage:   `Response: omit the "failed_responses" array if empty`,
					CommitMessages:  []string{"Response: omit the \"failed_responses\" array if empty"},
					PushCommitPaths: []bitriseapi.CommitPaths{{}},
					Branch:          "master",
					Environments:    []bitriseapi.EnvironmentItem{{Name: commitMessagesEnvKey, Value: "- Response: omit the \"failed_responses\" array if empty\n", IsExpand: false}},
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
		hookTransformResult := NewDefaultHookProvider(zap.NewNop()).transformCodePushEvent(codePush)
		require.NoError(t, hookTransformResult.Error)
		require.False(t, hookTransformResult.ShouldSkip)
		require.Equal(t, []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: bitriseapi.BuildParamsModel{
					CommitHash:      "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage:   `Response: omit the "failed_responses" array if empty`,
					CommitMessages:  []string{"switch to three component versions", "Response: omit the \"failed_responses\" array if empty", "get version : three component version"},
					PushCommitPaths: []bitriseapi.CommitPaths{{}, {}, {}},
					Branch:          "master",
					Environments:    []bitriseapi.EnvironmentItem{{Name: commitMessagesEnvKey, Value: "- switch to three component versions\n- Response: omit the \"failed_responses\" array if empty\n- get version : three component version\n", IsExpand: false}},
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Trim commit messages")
	{
		maxSize := envVarSizeLimitInByte()

		codePush := CodePushEventModel{
			ObjectKind:   "push",
			Ref:          "refs/heads/master",
			CheckoutSHA:  "7782203aaf0daabbd245ec0370c751eac6a4eb55",
			UserUsername: "test_user",
			Commits: []CommitModel{
				{
					CommitHash:    "7782203aaf0daabbd245ec0370c751eac6a4eb55",
					CommitMessage: generateText(maxSize),
				},
				{
					CommitHash:    "f8f37818dc89a67516adfc21896d0c9ec43d05c2",
					CommitMessage: generateText(maxSize),
				},
			},
		}

		hookTransformResult := NewDefaultHookProvider(zap.NewNop()).transformCodePushEvent(codePush)
		require.Equal(t, 1, len(hookTransformResult.TriggerAPIParams))

		triggerParam := hookTransformResult.TriggerAPIParams[0]
		require.Equal(t, 1, len(triggerParam.BuildParams.Environments))

		env := triggerParam.BuildParams.Environments[0]
		require.Equal(t, maxSize, len([]byte(env.Value)), env.Value)
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
		hookTransformResult := NewDefaultHookProvider(zap.NewNop()).transformCodePushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "The commit specified by 'checkout_sha' was not included in the 'commits' array - no match found")
		require.False(t, hookTransformResult.ShouldSkip)
		require.Nil(t, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Commit without CheckoutSHA (squashed merge request)")
	{
		codePush := CodePushEventModel{
			ObjectKind:   "push",
			Ref:          "refs/heads/master",
			CheckoutSHA:  "",
			UserUsername: "test_user",
			Commits:      []CommitModel{},
		}
		hookTransformResult := NewDefaultHookProvider(zap.NewNop()).transformCodePushEvent(codePush)
		require.EqualError(t, hookTransformResult.Error, "The 'checkout_sha' field is not set - potential squashed merge request")
		require.True(t, hookTransformResult.ShouldSkip)
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
		hookTransformResult := NewDefaultHookProvider(zap.NewNop()).transformCodePushEvent(codePush)
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

	t.Log("Update - irrelevant changes")
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

	t.Log("Update - draft published")
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
				Action: "update",
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
				MergeStatus: "unchecked",
			},
			Changes: Changes{
				Draft: BoolChanges{
					Previous: true,
					Current:  false,
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
					PullRequestID:            &intTwelve,
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "",
					PullRequestHeadBranch:    "merge-requests/12/head",
					PullRequestReadyState:    bitriseapi.PullRequestReadyStateConvertedToReadyForReview,
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
		require.Equal(t, true, hookTransformResult.DontWaitForTriggerResponse)
	}

	t.Log("Update - labels changed")
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
				Action: "update",
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
				MergeStatus: "unchecked",
			},
			Labels: []LabelInfoModel{
				{ID: 1, Title: "existing1"},
				{ID: 3, Title: "new1"},
				{ID: 4, Title: "new2"},
			},
			Changes: Changes{
				Labels: LabelChanges{
					Previous: []LabelInfoModel{
						{ID: 1, Title: "existing1"},
						{ID: 2, Title: "existing2"},
					},
					Current: []LabelInfoModel{
						{ID: 1, Title: "existing1"},
						{ID: 3, Title: "new1"},
						{ID: 4, Title: "new2"},
					},
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
					PullRequestID:            &intTwelve,
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "",
					PullRequestHeadBranch:    "merge-requests/12/head",
					PullRequestReadyState:    bitriseapi.PullRequestReadyStateReadyForReview,
					PullRequestLabels:        []string{"existing1", "new1", "new2"},
					NewPullRequestLabels:     []string{"new1", "new2"},
				},
				TriggeredBy: "webhook-gitlab/test_user",
			},
		}, hookTransformResult.TriggerAPIParams)
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
				MergeStatus: "unchecked",
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
					PullRequestID:            &intTwelve,
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "",
					PullRequestHeadBranch:    "merge-requests/12/head",
					PullRequestReadyState:    bitriseapi.PullRequestReadyStateReadyForReview,
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
					PullRequestID:            &intTwelve,
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestMergeBranch:   "merge-requests/12/merge",
					PullRequestHeadBranch:    "merge-requests/12/head",
					PullRequestReadyState:    bitriseapi.PullRequestReadyStateReadyForReview,
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
					CommitHash:     "1606d3dd4c4dc83ee8fed8d3cfd911da851bf740",
					CommitMessage:  "second commit message",
					CommitMessages: []string{"first commit message", "second commit message"},
					PushCommitPaths: []bitriseapi.CommitPaths{
						{
							Added:    []string{"README.MD"},
							Modified: []string{"app/controller/application.rb"},
							Removed:  []string{},
						},
						{
							Added:    []string{"CHANGELOG"},
							Modified: []string{"app/controller/application.rb"},
							Removed:  []string{},
						},
					},
					Branch:       "develop",
					Environments: []bitriseapi.EnvironmentItem{{Name: commitMessagesEnvKey, Value: "- first commit message\n- second commit message\n", IsExpand: false}},
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
					PullRequestID:            &intTwelve,
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					PullRequestAuthor:        "Author Name",
					PullRequestMergeBranch:   "",
					PullRequestHeadBranch:    "merge-requests/12/head",
					PullRequestReadyState:    bitriseapi.PullRequestReadyStateReadyForReview,
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
					PullRequestID:            &intTwelve,
					BaseRepositoryURL:        "https://gitlab.com/bitrise-io/bitrise-webhooks.git",
					HeadRepositoryURL:        "https://gitlab.com/oss-contributor/fork-bitrise-webhooks.git",
					PullRequestRepositoryURL: "https://gitlab.com/oss-contributor/fork-bitrise-webhooks.git",
					PullRequestAuthor:        "Author Name",
					PullRequestMergeBranch:   "merge-requests/12/merge",
					PullRequestHeadBranch:    "merge-requests/12/head",
					PullRequestReadyState:    bitriseapi.PullRequestReadyStateReadyForReview,
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

func Test_ensureCommitMessagesSize(t *testing.T) {
	tests := []struct {
		name           string
		maxSize        int
		commitMessages []string
		want           []string
	}{
		{
			name:           "First two messages needs to be trimmed",
			maxSize:        4 * len([]byte("1234567890")), // 4 * 10 bytes - 4 * 3 bytes (yaml control chars) = 28 bytes max
			commitMessages: []string{"123456789a", "123456789abc", "123a", "1a"},
			want:           []string{"1234...", "1234...", "123a", "1a"}, // 28 / 4 = 7 bytes max per message
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDefaultHookProvider(zap.NewNop()).ensureCommitMessagesSize(tt.commitMessages, tt.maxSize)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)

			require.True(t, messagesSize(got) <= tt.maxSize)
		})
	}
}

func Test_transformPullRequestEvent_readyState(t *testing.T) {
	tests := []struct {
		name           string
		pullRequest    MergeRequestEventModel
		wantReadyState bitriseapi.PullRequestReadyState
	}{
		{
			name: "Draft PR opened",
			pullRequest: MergeRequestEventModel{
				ObjectKind: "merge_request",
				ObjectAttributes: ObjectAttributesInfoModel{
					State:  "opened",
					Action: "open",
					Draft:  true,
				},
				Changes: Changes{
					Draft: BoolChanges{
						Previous: false,
						Current:  false,
					},
				},
			},
			wantReadyState: bitriseapi.PullRequestReadyStateDraft,
		},
		{
			name: "Draft PR updated with code push",
			pullRequest: MergeRequestEventModel{
				ObjectKind: "merge_request",
				ObjectAttributes: ObjectAttributesInfoModel{
					State:  "opened",
					Action: "update",
					Oldrev: "asdf",
					Draft:  true,
				},
				Changes: Changes{
					Draft: BoolChanges{
						Previous: false,
						Current:  false,
					},
				},
			},
			wantReadyState: bitriseapi.PullRequestReadyStateDraft,
		},
		{
			name: "Draft PR converted to ready to review PR",
			pullRequest: MergeRequestEventModel{
				ObjectKind: "merge_request",
				ObjectAttributes: ObjectAttributesInfoModel{
					State:  "opened",
					Action: "update",
					Draft:  false,
				},
				Changes: Changes{
					Draft: BoolChanges{
						Previous: true,
						Current:  false,
					},
				},
			},
			wantReadyState: bitriseapi.PullRequestReadyStateConvertedToReadyForReview,
		},
		{
			name: "Ready to review PR updated with code push",
			pullRequest: MergeRequestEventModel{
				ObjectKind: "merge_request",
				ObjectAttributes: ObjectAttributesInfoModel{
					State:  "opened",
					Action: "update",
					Oldrev: "asdf",
					Draft:  false,
				},
				Changes: Changes{
					Draft: BoolChanges{
						Previous: false,
						Current:  false,
					},
				},
			},
			wantReadyState: bitriseapi.PullRequestReadyStateReadyForReview,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformMergeRequestEvent(tt.pullRequest)
			require.Equal(t, 1, len(got.TriggerAPIParams))
			require.Equal(t, tt.wantReadyState, got.TriggerAPIParams[0].BuildParams.PullRequestReadyState)
		})
	}
}

func generateText(sizeInKB int) string {
	return strings.Repeat("a", sizeInKB*1000)
}

func messagesSize(messages []string) int {
	size := 0
	for _, message := range messages {
		size += len([]byte(message))
	}
	return size
}
