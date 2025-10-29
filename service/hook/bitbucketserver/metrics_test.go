package bitbucketserver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/stretchr/testify/require"
)

func TestHookProvider_gatherMetrics(t *testing.T) {
	currentTime := time.Date(2023, time.October, 26, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		metricsMethod func(eventKey string, appSlug string, currentTime time.Time) ([]common.Metrics, error)
		webhookType   string
		appSlug       string
		want          string
	}{
		{
			name: "Pull Request created webhook",
			metricsMethod: func(eventKey string, appSlug string, currentTime time.Time) ([]common.Metrics, error) {
				return HookProvider{}.gatherPRMetrics(testPullRequestCreatedWebhook(t), eventKey, appSlug, currentTime)
			},
			appSlug:     "slug",
			webhookType: "pr:opened",
			want: `{
	"event": "pull_request",
	"action": "opened",
	"provider_type": "bitbucket-server",
	"repository": "PROJ/repository",
	"timestamp": "2023-10-26T08:00:00Z",
	"event_timestamp": "2017-09-19T09:58:11+10:00",
	"app_slug": "slug",
	"original_trigger": "pr:opened:",
	"user_name": "admin",
	"git_ref": "a-branch",
	"pull_request_title": "a new file added",
	"pull_request_id": "1",
	"target_branch": "master",
	"commit_id": "ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
	"changed_files_count": 0,
	"addition_count": 0,
	"deletion_count": 0,
	"commit_count": 0,
	"status": "opened"
}
`,
		},
		{
			name: "Push webhook",
			metricsMethod: func(eventKey string, appSlug string, currentTime time.Time) ([]common.Metrics, error) {
				return HookProvider{}.gatherPushMetrics(testPushWebhook(t), eventKey, appSlug, currentTime)
			},
			appSlug:     "slug",
			webhookType: "repo:refs_changed",
			want: `{
	"event": "git_push",
	"action": "pushed",
	"provider_type": "bitbucket-server",
	"repository": "PROJECT_1/rep_1",
	"timestamp": "2023-10-26T08:00:00Z",
	"event_timestamp": "2023-01-13T22:26:25+11:00",
	"app_slug": "slug",
	"original_trigger": "repo:refs_changed:",
	"user_name": "admin",
	"git_ref": "master",
	"commit_id_after": "a00945762949b7b787ecabc388c0e20b1b85f0b4",
	"commit_id_before": "197a3e0d2f9a2b3ed1c4fe5923d5dd701bee9fdd",
	"changed_files_count": 0,
	"addition_count": 0,
	"deletion_count": 0
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.metricsMethod(tt.webhookType, tt.appSlug, currentTime)
			require.NoError(t, err)
			require.Equal(t, 1, len(got))
			got1 := got[0]
			gotBytes, err := got1.Serialise()
			require.NoError(t, err)
			require.Equal(t, compactJSON(tt.want), compactJSON(string(gotBytes)))
		})
	}
}

func testPushWebhook(t *testing.T) PushEventModel {
	var event PushEventModel
	err := json.Unmarshal([]byte(testPushWebhookPayload), &event)
	require.NoError(t, err)
	return event
}

func testPullRequestCreatedWebhook(t *testing.T) PullRequestEventModel {
	var event PullRequestEventModel
	err := json.Unmarshal([]byte(testPullRequestCreatedWebhookPayload), &event)
	require.NoError(t, err)
	return event
}

func compactJSON(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

const testPushWebhookPayload = `{
  "eventKey": "repo:refs_changed",
  "date": "2023-01-13T22:26:25+1100",
  "actor": {
    "name": "admin",
    "emailAddress": "admin@example.com",
    "active": true,
    "displayName": "Administrator",
    "id": 2,
    "slug": "admin",
    "type": "NORMAL"
    },
    "repository": {
      "slug": "rep_1",
      "id": 1,
      "name": "rep_1",
      "hierarchyId": "af05451fc6eb4bf4e0bd",
      "scmId": "git",
      "state": "AVAILABLE",
      "statusMessage": "Available",
      "forkable": true,
      "project": {
          "key": "PROJECT_1",
          "id": 1,
          "name": "Project 1",
          "description": "PROJECT_1",
          "public": false,
          "type": "NORMAL"
      },
      "public": false,
      "archived": false
    },
    "changes": [
      {
        "ref": {
          "id": "refs/heads/master",
          "displayId": "master",
          "type": "BRANCH"
        },
        "refId": "refs/heads/master",
        "fromHash": "197a3e0d2f9a2b3ed1c4fe5923d5dd701bee9fdd",
        "toHash": "a00945762949b7b787ecabc388c0e20b1b85f0b4",
        "type": "UPDATE"
      }
    ],
    "commits": [
      {
        "id": "a00945762949b7b787ecabc388c0e20b1b85f0b4",
        "displayId": "a0094576294",
        "author": {
          "name": "Administrator",
          "emailAddress": "admin@example.com"
        },
        "authorTimestamp": 1673403328000,
        "committer": {
          "name": "Administrator",
          "emailAddress": "admin@example.com"
        },
        "committerTimestamp": 1673403328000,
        "message": "My commit message",
        "parents": [
            {
              "id": "197a3e0d2f9a2b3ed1c4fe5923d5dd701bee9fdd",
              "displayId": "197a3e0d2f9"
            }
        ]
      }
    ],
    "toCommit": {
        "id": "a00945762949b7b787ecabc388c0e20b1b85f0b4",
        "displayId": "a0094576294",
        "author": {
            "name": "Administrator",
            "emailAddress": "admin@example.com"
        },
        "authorTimestamp": 1673403328000,
        "committer": {
            "name": "Administrator",
            "emailAddress": "admin@example.com"
        },
        "committerTimestamp": 1673403328000,
        "message": "My commit message",
        "parents": [
            {
              "id": "197a3e0d2f9a2b3ed1c4fe5923d5dd701bee9fdd",
              "displayId": "197a3e0d2f9",
              "author": {
                  "name": "Administrator",
                  "emailAddress": "admin@example.com"
              },
              "authorTimestamp": 1673403292000,
              "committer": {
                  "name": "Administrator",
                  "emailAddress": "admin@example.com"
              },
              "committerTimestamp": 1673403292000,
              "message": "My commit message",
              "parents": [
                  {
                    "id": "f870ce6bf6fe633e1a2bbe655970bde25535669f",
                    "displayId": "f870ce6bf6f"
                  }
              ]
            }
        ]
    }
} `

const testPullRequestCreatedWebhookPayload = `{
  "eventKey": "pr:opened",
  "date": "2017-09-19T09:58:11+1000",
  "actor": {
    "name": "admin",
    "emailAddress": "admin@example.com",
    "id": 1,
    "displayName": "Administrator",
    "active": true,
    "slug": "admin",
    "type": "NORMAL"
  },
  "pullRequest": {
    "id": 1,
    "version": 0,
    "title": "a new file added",
    "state": "OPEN",
    "open": true,
    "closed": false,
 	"draft": false,
    "createdDate": 1505779091796,
    "updatedDate": 1505779091796,
    "fromRef": {
      "id": "refs/heads/a-branch",
      "displayId": "a-branch",
      "latestCommit": "ef8755f06ee4b28c96a847a95cb8ec8ed6ddd1ca",
      "repository": {
        "slug": "repository",
        "id": 84,
        "name": "repository",
        "scmId": "git",
        "state": "AVAILABLE",
        "statusMessage": "Available",
        "forkable": true,
        "project": {
          "key": "PROJ",
          "id": 84,
          "name": "project",
          "public": false,
          "type": "NORMAL"
        },
        "public": false
      }
    },
    "toRef": {
      "id": "refs/heads/master",
      "displayId": "master",
      "latestCommit": "178864a7d521b6f5e720b386b2c2b0ef8563e0dc",
      "repository": {
        "slug": "repository",
        "id": 84,
        "name": "repository",
        "scmId": "git",
        "state": "AVAILABLE",
        "statusMessage": "Available",
        "forkable": true,
        "project": {
          "key": "PROJ",
          "id": 84,
          "name": "project",
          "public": false,
          "type": "NORMAL"
        },
        "public": false
      }
    },
    "locked": false,
    "author": {
      "user": {
        "name": "admin",
        "emailAddress": "admin@example.com",
        "id": 1,
        "displayName": "Administrator",
        "active": true,
        "slug": "admin",
        "type": "NORMAL"
      },
      "role": "AUTHOR",
      "approved": false,
      "status": "UNAPPROVED"
    },
    "reviewers": [

    ],
    "participants": [

    ],
    "links": {
      "self": [
        null
      ]
    }
  }
}`
