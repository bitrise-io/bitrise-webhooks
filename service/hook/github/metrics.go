package github

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/google/go-github/v54/github"
)

// GatherMetrics ...
func (hp HookProvider) GatherMetrics(r *http.Request) (measured bool, result common.MetricsResultModel) {
	payload, err := github.ValidatePayload(r, nil)
	if err != nil {
		return false, hookCommon.MetricsResultModel{}
	}

	webhookType := github.WebHookType(r)

	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		fmt.Println(err)
		return false, hookCommon.MetricsResultModel{}
	}

	var metrics interface{}

	switch event := event.(type) {
	case *github.PullRequestEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPullRequestOpenedAction(webhookType, event.GetAction()):
			fmt.Printf("PR opened: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestOpenedMetrics(event, webhookType) // OK
			fmt.Println(metrics)
		case isPullRequestUpdatedAction(webhookType, event.GetAction()):
			fmt.Printf("PR updated: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestUpdatedMetrics(event, webhookType) // OK
			fmt.Println(metrics)
		case isPullRequestCommentAction(webhookType, event.GetAction()):
			fmt.Printf("PR comment: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestCommentMetrics(event, webhookType) // OK
			fmt.Println(metrics)
		case isPullRequestClosedAction(webhookType, event.GetAction()):
			fmt.Printf("PR closed: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestClosedMetrics(event, webhookType) // OK
			fmt.Println(metrics)
		}

	case *github.PullRequestReviewEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPullRequestUpdatedAction(webhookType, event.GetAction()):
			fmt.Printf("PR updated: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestUpdatedMetrics(event, webhookType) // OK
			fmt.Println(metrics)
		}
	case *github.PullRequestReviewCommentEvent: // OK
		fmt.Println("action:", event.GetAction())

		switch {
		case isPullRequestCommentAction(webhookType, event.GetAction()):
			fmt.Printf("PR comment: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestCommentMetrics(event, webhookType) // OK
			fmt.Println(metrics)
		}
	case *github.PullRequestReviewThreadEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPullRequestCommentAction(webhookType, event.GetAction()):
			fmt.Printf("PR comment: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestCommentMetrics(event, webhookType) // OK
			fmt.Println(metrics)
		}
	case *github.PushEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPushAction(webhookType, event.GetAction()):
			fmt.Printf("Push: %s:%s\n", webhookType, event.GetAction())
			metrics = newPushMetrics(event, webhookType) // OK
			fmt.Println(metrics)
		}
	}

	return true, hookCommon.MetricsResultModel{}
}

func newPushMetrics(event *github.PushEvent, webhookType string) common.PushMetrics {
	createdAt := event.GetHeadCommit().GetTimestamp()
	timestamp := createdAt.GetTime() // TODO: branch delete -> timestamp empty
	action := event.GetAction()
	originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)
	commits := event.GetCommits()
	var oldestCommit *github.HeadCommit
	if len(commits) > 0 {
		oldestCommit = commits[0]
	}
	var oldestCommitTimestamp *time.Time
	if oldestCommit != nil {
		t := oldestCommit.GetTimestamp()
		oldestCommitTimestamp = t.GetTime()
	}

	// branch delete event:
	// - CommitIDBefore:
	// - CommitIDAfter: null
	// branch create event:
	// - CommitIDBefore: nul
	// - CommitIDAfter: <commit_id>
	return common.PushMetrics{
		GeneralMetrics: common.GeneralMetrics{
			Timestamp:       *timestamp,
			AppSlug:         "",
			Action:          action,
			OriginalTrigger: originalTrigger,
			Email:           event.GetPusher().GetEmail(),
			Username:        event.GetPusher().GetName(),
			GitRef:          event.GetRef(),
		},
		CommitIDBefore:        event.GetBefore(),
		CommitIDAfter:         event.GetAfter(),
		OldestCommitTimestamp: oldestCommitTimestamp,
	}
}

func newPullRequestMetrics(pullRequest *github.PullRequest, webhookType, action string) common.PullRequestMetrics {
	prID := fmt.Sprintf("%d", pullRequest.GetNumber())

	return hookCommon.PullRequestMetrics{
		PullRequestID:         prID,
		CommitID:              pullRequest.GetHead().GetSHA(),
		OldestCommitTimestamp: nil,
		ChangedFiles:          pullRequest.GetChangedFiles(),
		Additions:             pullRequest.GetAdditions(),
		Deletions:             pullRequest.GetDeletions(),
		Commits:               pullRequest.GetCommits(),
	}
}

func newPullRequestOpenedMetrics(event interface{}, webhookType string) common.PullRequestOpenedMetrics {
	switch event := event.(type) {
	case *github.PullRequestEvent:
		pullRequest := event.GetPullRequest()
		createdAt := pullRequest.GetCreatedAt()
		timestamp := createdAt.GetTime()
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return common.PullRequestOpenedMetrics{
			GeneralMetrics: common.GeneralMetrics{
				Timestamp:       *timestamp,
				AppSlug:         "",
				Action:          action,
				OriginalTrigger: originalTrigger,
				Email:           pullRequest.GetUser().GetEmail(),
				Username:        pullRequest.GetUser().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(event.GetPullRequest(), webhookType, event.GetAction()),
			Status:             event.GetPullRequest().GetState(),
		}
	}

	return common.PullRequestOpenedMetrics{}
}

func newPullRequestUpdatedMetrics(event interface{}, webhookType string) common.PullRequestUpdatedMetrics {
	switch event := event.(type) {
	case *github.PullRequestEvent:
		pullRequest := event.GetPullRequest()
		updatedAt := pullRequest.GetUpdatedAt()
		timestamp := updatedAt.GetTime()
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return common.PullRequestUpdatedMetrics{
			GeneralMetrics: common.GeneralMetrics{
				Timestamp:       *timestamp,
				AppSlug:         "",
				Action:          action,
				OriginalTrigger: originalTrigger,
				Email:           event.GetSender().GetEmail(),
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(pullRequest, webhookType, event.GetAction()),
			Status:             event.GetPullRequest().GetState(),
		}
	case *github.PullRequestReviewEvent:
		pullRequest := event.GetPullRequest()
		updatedAt := pullRequest.GetUpdatedAt()
		timestamp := updatedAt.GetTime()
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return common.PullRequestUpdatedMetrics{
			GeneralMetrics: common.GeneralMetrics{
				Timestamp:       *timestamp,
				AppSlug:         "",
				Action:          action,
				OriginalTrigger: originalTrigger,
				Email:           event.GetSender().GetEmail(),
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(pullRequest, webhookType, event.GetAction()),
			Status:             event.GetPullRequest().GetState(),
		}
	}
	return common.PullRequestUpdatedMetrics{}
}

func newPullRequestCommentMetrics(event interface{}, webhookType string) common.PullRequestCommentMetrics {
	switch event := event.(type) {
	case *github.PullRequestReviewCommentEvent:
		pullRequest := event.GetPullRequest()
		comment := event.GetComment()
		createdAt := comment.GetCreatedAt()
		timestamp := createdAt.GetTime()
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return common.PullRequestCommentMetrics{
			GeneralMetrics: common.GeneralMetrics{
				Timestamp:       *timestamp,
				AppSlug:         "",
				Action:          action,
				OriginalTrigger: originalTrigger,
				Email:           event.GetSender().GetEmail(),
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(event.GetPullRequest(), webhookType, event.GetAction()),
		}
	case *github.PullRequestReviewThreadEvent:
		pullRequest := event.GetPullRequest()
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return common.PullRequestCommentMetrics{
			GeneralMetrics: common.GeneralMetrics{
				//Timestamp:       *timestamp, // TODO: what should be the timestamp here?
				AppSlug:         "",
				Action:          action,
				OriginalTrigger: originalTrigger,
				Email:           event.GetSender().GetEmail(),
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(event.GetPullRequest(), webhookType, event.GetAction()),
		}
	}
	return common.PullRequestCommentMetrics{}
}

func newPullRequestClosedMetrics(event interface{}, webhookType string) common.PullRequestClosedMetrics {
	switch event := event.(type) {
	case *github.PullRequestEvent:
		pullRequest := event.GetPullRequest()
		updatedAt := pullRequest.GetUpdatedAt()
		timestamp := updatedAt.GetTime()
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return common.PullRequestClosedMetrics{
			GeneralMetrics: common.GeneralMetrics{
				Timestamp:       *timestamp,
				AppSlug:         "",
				Action:          action,
				OriginalTrigger: originalTrigger,
				Email:           event.GetSender().GetEmail(),
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(event.GetPullRequest(), webhookType, event.GetAction()),
			Status:             event.GetPullRequest().GetState(),
		}
	}
	return common.PullRequestClosedMetrics{}
}

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
	"push": {
		"",
	},
}

func isPushAction(event, action string) bool {
	supportedActions := pushActions[event]
	return sliceutil.IsStringInSlice(action, supportedActions)
}
