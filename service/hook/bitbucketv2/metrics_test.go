package bitbucketv2

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/webhooks/v6/bitbucket"
	"github.com/stretchr/testify/require"
)

func TestHookProvider_gatherMetrics(t *testing.T) {
	currentTime := time.Date(2023, time.October, 26, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		event       interface{}
		webhookType string
		appSlug     string
		want        string
	}{
		{
			name:        "Pull Request created webhook",
			event:       testPullRequestCreatedWebhook(t),
			appSlug:     "slug",
			webhookType: "pullrequest:created",
			want: `{
	"event": "pull_request",
	"action": "opened",
	"provider_type": "bitbucket-v2",
	"repository": "bitrise-io/project",
	"timestamp": "2023-10-26T08:00:00Z",
	"event_timestamp": "2023-11-08T13:18:53.923474Z",
	"app_slug": "slug",
	"original_trigger": "pullrequest:created:",
	"user_name": "bitrise-bot",
	"git_ref": "dev",
	"pull_request_title": "README.md edited online with Bitbucket",
	"pull_request_id": "10",
	"pull_request_url": "https://bitbucket.org/bitrise-io/project/pull-requests/10",
	"target_branch": "master",
	"commit_id": "66980da5d45c",
	"changed_files_count": 0,
	"addition_count": 0,
	"deletion_count": 0,
	"commit_count": 0,
	"status": "opened"
}
`,
		},
		{
			name:        "Push webhook",
			event:       testPushWebhook(t),
			appSlug:     "slug",
			webhookType: "repo:push",
			want: `{
	"event": "git_push",
	"action": "forced",
	"provider_type": "bitbucket-v2",
	"repository": "bitrise-io/project",
	"timestamp": "2023-10-26T08:00:00Z",
	"event_timestamp": "2023-11-08T13:26:03Z",
	"app_slug": "slug",
	"original_trigger": "repo:push:",
	"user_name": "bitrise-bot",
	"git_ref": "dev",
	"commit_id_after": "303f1d584a4299f6e1dabb58731ab2026ef70e05",
	"commit_id_before": "51cdb0ee811d801a73a099b2e4f011e8d2c98efa",
	"changed_files_count": 0,
	"addition_count": 0,
	"deletion_count": 0
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp := HookProvider{}
			got := hp.gatherMetrics(tt.event, tt.webhookType, tt.appSlug, currentTime)
			require.Equal(t, 1, len(got))
			got1 := got[0]
			gotBytes, err := got1.Serialise()
			require.NoError(t, err)
			require.Equal(t, compactJSON(tt.want), compactJSON(string(gotBytes)))
		})
	}
}

func compactJSON(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func testPushWebhook(t *testing.T) interface{} {
	var event bitbucket.RepoPushPayload
	err := json.Unmarshal([]byte(testPushWebhookPayload), &event)
	require.NoError(t, err)
	return event
}

const testPushWebhookPayload = `{
  "push": {
    "changes": [
      {
        "old": {
          "name": "dev",
          "target": {
            "type": "commit",
            "hash": "51cdb0ee811d801a73a099b2e4f011e8d2c98efa",
            "date": "2023-11-08T13:19:28+00:00"
          },
          "type": "branch"
        },
        "new": {
          "name": "dev",
          "target": {
            "type": "commit",
            "hash": "303f1d584a4299f6e1dabb58731ab2026ef70e05",
            "date": "2023-11-08T13:26:03+00:00"
          },
          "type": "branch"
        },
        "created": false,
        "forced": true,
        "closed": false
      }
    ]
  },
  "repository": {
    "full_name": "bitrise-io/project"
  },
  "actor": {
    "nickname": "bitrise-bot"
  }
}`

func testPullRequestCreatedWebhook(t *testing.T) interface{} {
	var event bitbucket.PullRequestCreatedPayload
	err := json.Unmarshal([]byte(testPullRequestCreatedWebhookPayload), &event)
	require.NoError(t, err)
	return event
}

const testPullRequestCreatedWebhookPayload = `{
    "repository": {
      "full_name": "bitrise-io/project"
    },
    "actor": {
      "nickname": "bitrise-bot"
    },
    "pullrequest": {
      "id": 10,
      "title": "README.md edited online with Bitbucket",
      "state": "OPEN",
      "merge_commit": null,
      "reason": "",
      "created_on": "2023-11-08T13:18:53.923474+00:00",
      "updated_on": "2023-11-08T13:18:55.372722+00:00",
      "destination": {
        "branch": {
          "name": "master"
        }
      },
      "source": {
        "branch": {
          "name": "dev"
        },
        "commit": {
          "hash": "66980da5d45c"
        }
      },
      "links": {
        "html": {
          "href": "https://bitbucket.org/bitrise-io/project/pull-requests/10"
        }
      }
    }
  }`
