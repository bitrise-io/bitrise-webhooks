package gitlab

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/xanzy/go-gitlab"
)

// GatherMetrics ...
func (hp HookProvider) GatherMetrics(r *http.Request, appSlug string) (common.Metrics, error) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	webhookType := gitlab.HookEventType(r)
	event, err := gitlab.ParseWebhook(webhookType, payload)
	if err != nil {
		return nil, err
	}

	currentTime := hp.timeProvider.CurrentTime()
	return hp.gatherMetrics(event, appSlug, currentTime), nil
}

func (hp HookProvider) gatherMetrics(event interface{}, appSlug string, currentTime time.Time) common.Metrics {
	var metrics common.Metrics
	switch event := event.(type) {
	case *gitlab.PushEvent:
		metrics = newPushMetrics(event, appSlug, currentTime)
	case *gitlab.MergeEvent:
		metrics = newPullRequestMetrics(event, appSlug, currentTime)
	}

	fmt.Println(metrics)
	return metrics
}

func newPullRequestMetrics(event *gitlab.MergeEvent, appSlug string, currentTime time.Time) common.PullRequestMetrics {
	var constructorFunc func(generalMetrics common.GeneralMetrics, generalPullRequestMetrics common.GeneralPullRequestMetrics) common.PullRequestMetrics

	switch event.ObjectAttributes.Action {
	case "open":
		constructorFunc = common.NewPullRequestOpenedMetrics
	case "close", "merge":
		constructorFunc = common.NewPullRequestClosedMetrics
	default:
		constructorFunc = common.NewPullRequestUpdatedMetrics
	}

	provider := ProviderID
	repo := event.Project.PathWithNamespace
	timestamp := parseTime(event.ObjectAttributes.UpdatedAt)
	originalTrigger := common.OriginalTrigger(event.EventType, event.ObjectAttributes.Action)
	userName := event.User.Username
	gitRef := fmt.Sprintf("refs/heads/%s", event.ObjectAttributes.SourceBranch)
	generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, timestamp, appSlug, originalTrigger, userName, gitRef)

	pullRequest := event
	generalPullRequestMetrics := newGeneralPullRequestMetrics(pullRequest)

	metrics := constructorFunc(generalMetrics, generalPullRequestMetrics)
	return metrics
}

func newPushMetrics(event *gitlab.PushEvent, appSlug string, currentTime time.Time) common.PushMetrics {
	var constructorFunc func(generalMetrics common.GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) common.PushMetrics

	switch {
	case isBranchCreate(event):
		constructorFunc = common.NewPushCreatedMetrics
	case isBranchDelete(event):
		constructorFunc = common.NewPushDeletedMetrics
	default:
		constructorFunc = common.NewPushMetrics
	}

	provider := ProviderID
	repo := event.Project.PathWithNamespace
	timestamp := (*time.Time)(nil)
	originalTrigger := common.OriginalTrigger(event.EventName, "")
	userName := event.UserUsername
	gitRef := event.Ref

	generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, timestamp, appSlug, originalTrigger, userName, gitRef)

	commitIDAfter := event.After
	commitIDBefore := event.Before
	oldestCommitTime := oldestCommitTimestamp(event)
	latestCommitTime := latestCommitTimestamp(event)
	masterBranch := event.Project.DefaultBranch

	return constructorFunc(generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTime, latestCommitTime, masterBranch)
}

func newGeneralPullRequestMetrics(pullRequest *gitlab.MergeEvent) common.GeneralPullRequestMetrics {
	prID := fmt.Sprintf("%d", pullRequest.ObjectAttributes.IID)

	return common.GeneralPullRequestMetrics{
		PullRequestTitle: pullRequest.ObjectAttributes.Title,
		PullRequestID:    prID,
		PullRequestURL:   pullRequest.ObjectAttributes.URL,
		TargetBranch:     pullRequest.ObjectAttributes.TargetBranch,
		CommitID:         pullRequest.ObjectAttributes.LastCommit.ID,
		MergeCommitSHA:   pullRequest.ObjectAttributes.MergeCommitSHA,
		Status:           pullRequest.ObjectAttributes.State,
	}
}

func isBranchCreate(event *gitlab.PushEvent) bool {
	return event.Before == "0000000000000000000000000000000000000000"
}

func isBranchDelete(event *gitlab.PushEvent) bool {
	return event.After == "0000000000000000000000000000000000000000"
}

func oldestCommitTimestamp(event *gitlab.PushEvent) *time.Time {
	if len(event.Commits) > 0 {
		return event.Commits[0].Timestamp
	}
	return nil
}

func latestCommitTimestamp(event *gitlab.PushEvent) *time.Time {
	if len(event.Commits) > 0 {
		return event.Commits[len(event.Commits)-1].Timestamp
	}
	return nil
}

func parseTime(s string) *time.Time {
	// 2023-10-19 11:50:00 UTC
	t, err := time.Parse("2006-01-02 03:04:05 MST", s)
	if err != nil {
		return nil
	}
	return &t
}
