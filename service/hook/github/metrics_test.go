package github

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/google/go-github/v55/github"
	"github.com/stretchr/testify/require"
)

func TestHookProvider_gatherMetrics_commit_id_before_and_after(t *testing.T) {
	currentTime := time.Date(2023, time.October, 26, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		event       interface{}
		webhookType string
		appSlug     string
		want        string
	}{
		{
			name:        "Push deleted webhook - commit id after is null, before isn't",
			event:       testPushDeletedWebhook(t),
			webhookType: "git-push",
			appSlug:     "slug",
			want:        `{"event":"git_push","action":"deleted","provider_type":"github","repository":"bitrise-io/project","timestamp":"2023-10-26T08:00:00Z","app_slug":"slug","original_trigger":"git-push:","user_name":"bitrise-bot","git_ref":"refs/heads/tech_improvements","commit_id_after":"0000000000000000000000000000000000000000","commit_id_before":"123ddfe9f740fb229b9cff3e43a484bbcedf7fa8","master_branch":"main","changed_files_count":0,"addition_count":0,"deletion_count":0}`,
		},
		{
			name:        "Pull Request opened webhook - open status gets transformed to opened",
			event:       testPullRequestOpenedPayload(t),
			webhookType: "pull_request",
			appSlug:     "slug",
			want:        `{"event":"pull_request","action":"opened","provider_type":"github","repository":"bitrise-io/project","timestamp":"2023-10-26T08:00:00Z","event_timestamp":"2023-10-30T09:48:35Z","app_slug":"slug","original_trigger":"pull_request:opened","user_name":"bitrise-bot","git_ref":"tech_improvements","pull_request_title":"Patch","pull_request_id":"5","pull_request_url":"https://github.com/bitrise-io/project/pull/5","target_branch":"master","commit_id":"11d6f5e55831ab3586f032393aaee1e942caef6e","changed_files_count":1,"addition_count":1,"deletion_count":1,"commit_count":2,"status":"opened"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp := HookProvider{}
			got := hp.gatherMetrics(tt.event, tt.webhookType, tt.appSlug, currentTime)
			gotBytes, err := got.Serialise()
			require.NoError(t, err)
			require.Equal(t, tt.want, string(gotBytes))
		})
	}

}

func TestHookProvider_gatherMetrics(t *testing.T) {
	currentTime := time.Now()

	tests := []struct {
		name        string
		event       interface{}
		webhookType string
		appSlug     string
		want        common.Metrics
	}{
		{
			name:        "Push event transformed to push metrics",
			event:       &github.PushEvent{},
			webhookType: "git-push",
			appSlug:     "slug",
			want: &common.PushMetrics{
				Event:  "git_push",
				Action: "pushed",
				GeneralMetrics: common.GeneralMetrics{
					ProviderType:    ProviderID,
					TimeStamp:       currentTime,
					AppSlug:         "slug",
					OriginalTrigger: "git-push:",
				},
			},
		},
		{
			name:        "Pull Request event transformed to Pull Request metrics",
			event:       &github.PullRequestEvent{},
			webhookType: "pull_request",
			appSlug:     "slug",
			want: &common.PullRequestMetrics{
				Event:  "pull_request",
				Action: "updated",
				GeneralMetrics: common.GeneralMetrics{
					ProviderType:    ProviderID,
					TimeStamp:       currentTime,
					AppSlug:         "slug",
					OriginalTrigger: "pull_request:",
				},
				GeneralPullRequestMetrics: common.GeneralPullRequestMetrics{
					PullRequestID: "0",
				},
			},
		},
		{
			name:        "Pull Request Review Comment event transformed to Pull Request Comment metrics",
			event:       &github.PullRequestReviewCommentEvent{},
			webhookType: "pull_request_review_comment",
			appSlug:     "slug",
			want: &common.PullRequestCommentMetrics{
				Event:  "pull_request",
				Action: "comment",
				GeneralMetrics: common.GeneralMetrics{
					ProviderType:    ProviderID,
					TimeStamp:       currentTime,
					AppSlug:         "slug",
					OriginalTrigger: "pull_request_review_comment:",
				},
				PullRequestID: "0",
			},
		},
		{
			name:        "Fork event is not supported",
			event:       &github.ForkEvent{},
			webhookType: "fork",
			appSlug:     "slug",
			want:        nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp := HookProvider{}
			got := hp.gatherMetrics(tt.event, tt.webhookType, tt.appSlug, currentTime)
			require.Equal(t, tt.want, got)
		})
	}
}

func testPushDeletedWebhook(t *testing.T) interface{} {
	var event github.PushEvent
	err := json.Unmarshal([]byte(pushDeletedWebhookPayload), &event)
	require.NoError(t, err)
	return &event
}

func testPullRequestOpenedPayload(t *testing.T) interface{} {
	var event github.PullRequestEvent
	err := json.Unmarshal([]byte(pullRequestOpenedPayload), &event)
	require.NoError(t, err)
	return &event
}

const pushDeletedWebhookPayload = `{
  "ref": "refs/heads/tech_improvements",
  "before": "123ddfe9f740fb229b9cff3e43a484bbcedf7fa8",
  "after": "0000000000000000000000000000000000000000",
  "repository": {
    "full_name": "bitrise-io/project",
    "default_branch": "main",
    "master_branch": "main"
  },
  "pusher": {
    "name": "bitrise-bot"
  },
  "sender": {
    "login": "bitrise-bot"
  },
  "created": false,
  "deleted": true,
  "forced": false,
  "base_ref": null,
  "commits": [

  ],
  "head_commit": null
}`

const pullRequestOpenedPayload = `{
	"action": "opened",
	"number": 5,
	"pull_request": {
		"html_url": "https://github.com/bitrise-io/project/pull/5",
		"number": 5,
		"state": "open",
		"title": "Patch",
		"user": {
			"login": "bitrise-bot"
		},
		"created_at": "2023-10-30T09:48:35Z",
		"updated_at": "2023-10-30T09:48:35Z",
		"merge_commit_sha": null,
		"draft": true,
		"head": {
			"ref": "tech_improvements",
			"sha": "11d6f5e55831ab3586f032393aaee1e942caef6e"
		},
		"base": {
			"ref": "master",
			"sha": "11fc26fbb995653f4a5e45b0d62516d4afbd0627"
		},
		"commits": 2,
		"additions": 1,
		"deletions": 1,
		"changed_files": 1
	},
	"repository": {
		"full_name": "bitrise-io/project",
		"default_branch": "master"
	}
}`
