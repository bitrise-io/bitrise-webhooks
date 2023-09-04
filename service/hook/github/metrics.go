package github

import (
	"fmt"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/google/go-github/v54/github"
)

var pullRequestOpenedTriggers = map[string][]string{
	"pull_request": {
		"opened",
		"reopened",
	},
}

func isPullRequestOpenedAction(event, action string) bool {
	supportedActions := pullRequestOpenedTriggers[event]
	return sliceutil.IsStringInSlice(action, supportedActions)
}

var pullRequestUpdatedTriggers = map[string][]string{
	"pull_request": {
		"synchronize",
		"edited",
		"assigned",
		"unassigned",
		"auto_merge_disabled",
		"auto_merge_enabled",
		"converted_to_draft",
		"ready_for_review",
		"enqueued",
		"dequeued",
		"labeled",
		"unlabeled",
		"locked",
		"unlocked",
		"milestoned",
		"demilestoned",
		"review_request_removed",
		"review_requested",
	},
	"pull_request_review": {
		"submitted",
	},
}

func isPullRequestUpdatedAction(event, action string) bool {
	supportedActions := pullRequestUpdatedTriggers[event]
	return sliceutil.IsStringInSlice(action, supportedActions)
}

var pullRequestCommentTriggers = map[string][]string{
	"pull_request_review_comment": {
		"created",
		"edited",
		"deleted",
	},
	"pull_request_review_thread": {
		"resolved",
		"unresolved",
	},
}

func isPullRequestCommentAction(event, action string) bool {
	supportedActions := pullRequestCommentTriggers[event]
	return sliceutil.IsStringInSlice(action, supportedActions)
}

var pullRequestClosedActions = map[string][]string{
	"pull_request": {
		"closed",
	},
}

func isPullRequestClosedAction(event, action string) bool {
	supportedActions := pullRequestClosedActions[event]
	return sliceutil.IsStringInSlice(action, supportedActions)
}

var pushActions = map[string][]string{
	"git_push": {
		"push",
	},
}

func isPushAction(event, action string) bool {
	supportedActions := pushActions[event]
	return sliceutil.IsStringInSlice(action, supportedActions)
}

// GatherMetrics ...
func (hp HookProvider) GatherMetrics(r *http.Request) (measured bool, result common.MetricsResultModel) {
	fmt.Println("GatherMetrics")

	payload, err := github.ValidatePayload(r, nil)
	if err != nil {
		return false, hookCommon.MetricsResultModel{}
	}

	webhookType := github.WebHookType(r)
	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		return false, hookCommon.MetricsResultModel{}
	}

	switch event := event.(type) {
	case *github.PullRequestEvent:
		switch {
		case isPullRequestOpenedAction(webhookType, *event.Action):
			fmt.Printf("PR opened: %s:%s\n", webhookType, *event.Action)
		case isPullRequestUpdatedAction(webhookType, *event.Action):
			fmt.Printf("PR updated: %s:%s\n", webhookType, *event.Action)
		case isPullRequestCommentAction(webhookType, *event.Action):
			fmt.Printf("PR comment: %s:%s\n", webhookType, *event.Action)
		case isPullRequestClosedAction(webhookType, *event.Action):
			fmt.Printf("PR closed: %s:%s\n", webhookType, *event.Action)
		}
	case *github.PullRequestReviewEvent:
		switch {
		case isPullRequestUpdatedAction(webhookType, *event.Action):
			fmt.Printf("PR updated: %s:%s\n", webhookType, *event.Action)
		}
	case *github.PullRequestReviewCommentEvent:
		switch {
		case isPullRequestCommentAction(webhookType, *event.Action):
			fmt.Printf("PR comment: %s:%s\n", webhookType, *event.Action)
		}
	case *github.PullRequestReviewThreadEvent:
		switch {
		case isPullRequestCommentAction(webhookType, *event.Action):
			fmt.Printf("PR comment: %s:%s\n", webhookType, *event.Action)
		}
	}

	return true, hookCommon.MetricsResultModel{}
}
