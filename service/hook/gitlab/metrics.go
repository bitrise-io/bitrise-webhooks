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
	return hp.gatherMetrics(event, webhookType, appSlug, currentTime), nil
}

func (hp HookProvider) gatherMetrics(event interface{}, webhookType gitlab.EventType, appSlug string, currentTime time.Time) common.Metrics {
	var metrics common.Metrics
	switch event := event.(type) {
	case *gitlab.PushEvent:
		fmt.Println("action:", event.EventName)
		metrics = newPushMetrics(event, event.ObjectKind, appSlug)
	}

	return metrics
}

func newPushMetrics(event *gitlab.PushEvent, webhookType, appSlug string) *common.PushMetrics {
	return nil
}
