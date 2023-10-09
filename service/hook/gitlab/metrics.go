package gitlab

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/xanzy/go-gitlab"
)

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
	}

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

	timestamp := (*time.Time)(nil)
	originalTrigger := fmt.Sprintf("%s:%s", event.EventName, "")
	userName := event.UserUsername
	gitRef := event.Ref

	generalMetrics := common.NewGeneralMetrics(currentTime, timestamp, appSlug, originalTrigger, userName, gitRef)

	commitIDAfter := event.After
	commitIDBefore := event.Before
	oldestCommitTime := oldestCommitTimestamp(event)
	latestCommitTime := latestCommitTimestamp(event)
	masterBranch := event.Project.DefaultBranch

	return constructorFunc(generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTime, latestCommitTime, masterBranch)
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
