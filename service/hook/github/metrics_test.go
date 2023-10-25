package github

import (
	"testing"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/stretchr/testify/require"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

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
