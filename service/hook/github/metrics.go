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
// TODO: remove debug logging
// TODO: shouldn't we return and log errors?
func (hp HookProvider) GatherMetrics(r *http.Request, appSlug string) (metrics common.Metrics) {
	payload, err := github.ValidatePayload(r, nil)
	if err != nil {
		return nil
	}

	webhookType := github.WebHookType(r)

	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	switch event := event.(type) {

	case *github.PullRequestEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPullRequestOpenedAction(webhookType, event.GetAction()):
			fmt.Printf("PR opened: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestOpenedMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		case isPullRequestUpdatedAction(webhookType, event.GetAction()):
			fmt.Printf("PR updated: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestUpdatedMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		case isPullRequestCommentAction(webhookType, event.GetAction()):
			fmt.Printf("PR comment: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestCommentMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		case isPullRequestClosedAction(webhookType, event.GetAction()):
			fmt.Printf("PR closed: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestClosedMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		}

	case *github.PullRequestReviewEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPullRequestUpdatedAction(webhookType, event.GetAction()):
			fmt.Printf("PR updated: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestUpdatedMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		}

	case *github.PullRequestReviewCommentEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPullRequestCommentAction(webhookType, event.GetAction()):
			fmt.Printf("PR comment: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestCommentMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		}

	case *github.PullRequestReviewThreadEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPullRequestCommentAction(webhookType, event.GetAction()):
			fmt.Printf("PR comment: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestCommentMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		}

	case *github.IssueCommentEvent:
		switch {
		case isPullRequestCommentAction(webhookType, event.GetAction()):
			fmt.Printf("PR comment: %s:%s\n", webhookType, event.GetAction())
			metrics = newPullRequestCommentMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		}

	case *github.PushEvent:
		fmt.Println("action:", event.GetAction())

		switch {
		case isPushAction(webhookType, event.GetAction()):
			fmt.Printf("Push: %s:%s\n", webhookType, event.GetAction())
			metrics = newPushMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		}

	case *github.DeleteEvent:
		fmt.Println("action:", "deleted")

		switch {
		case isPushAction(webhookType, ""):
			fmt.Printf("Push: %s:%s\n", webhookType, "deleted")
			metrics = newPushMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		}

	case *github.CreateEvent:
		fmt.Println("action:", "created")

		switch {
		case isPushAction(webhookType, ""):
			fmt.Printf("Push: %s:%s\n", webhookType, "created")
			metrics = newPushMetrics(event, webhookType, appSlug)
			fmt.Println(metrics)
		}
	}

	return metrics
}

func newPushMetrics(event interface{}, webhookType, appSlug string) *common.PushMetrics {
	switch event := event.(type) {
	case *github.PushEvent:
		timestamp := timestampToTime(event.GetHeadCommit().GetTimestamp())
		oldestCommitTime := oldestCommitTimestamp(event.GetCommits())

		action := event.GetAction()
		if action == "" {
			switch webhookType {
			case "push":
				switch {
				case event.GetCreated():
					action = "created"
				case event.GetDeleted():
					action = "deleted"
				case event.GetForced():
					action = "forced"
				default:
					action = "pushed"
				}
			}
		}
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		// commit delete push:
		// - CommitIDBefore:
		// - CommitIDAfter: null
		// new commit push:
		// - CommitIDBefore: nul
		// - CommitIDAfter: <commit_id>
		return &common.PushMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  timestamp,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetPusher().GetName(),
				GitRef:          event.GetRef(),
			},
			CommitIDBefore:        event.GetBefore(),
			CommitIDAfter:         event.GetAfter(),
			OldestCommitTimestamp: oldestCommitTime,
		}
	case *github.DeleteEvent:
		action := "deleted"
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)
		return &common.PushMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  nil,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetSender().GetLogin(),
				GitRef:          event.GetRef(),
			},
		}
	case *github.CreateEvent:
		action := "created"
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)
		return &common.PushMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  nil,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetSender().GetLogin(),
				GitRef:          event.GetRef(),
			},
			MasterBranch: event.GetMasterBranch(),
		}
	}

	return nil
}

func newPullRequestOpenedMetrics(event interface{}, webhookType, appSlug string) *common.PullRequestOpenedMetrics {
	switch event := event.(type) {
	case *github.PullRequestEvent:
		pullRequest := event.GetPullRequest()
		timestamp := timestampToTime(pullRequest.GetCreatedAt())
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return &common.PullRequestOpenedMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  timestamp,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        pullRequest.GetUser().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(event.GetPullRequest()),
			Status:             event.GetPullRequest().GetState(),
		}
	}

	return nil
}

func newPullRequestUpdatedMetrics(event interface{}, webhookType, appSlug string) *common.PullRequestUpdatedMetrics {
	switch event := event.(type) {
	case *github.PullRequestEvent:
		pullRequest := event.GetPullRequest()
		timestamp := timestampToTime(pullRequest.GetUpdatedAt())
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return &common.PullRequestUpdatedMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  timestamp,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(pullRequest),
			Status:             event.GetPullRequest().GetState(),
		}
	case *github.PullRequestReviewEvent:
		pullRequest := event.GetPullRequest()
		timestamp := timestampToTime(pullRequest.GetUpdatedAt())
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		return &common.PullRequestUpdatedMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  timestamp,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: newPullRequestMetrics(pullRequest),
			Status:             event.GetPullRequest().GetState(),
		}
	}
	return nil
}

func newPullRequestCommentMetrics(event interface{}, webhookType, appSlug string) *common.PullRequestCommentMetrics {
	switch event := event.(type) {
	case *github.PullRequestReviewCommentEvent:
		comment := event.GetComment()
		timestamp := timestampToTime(comment.GetCreatedAt())
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)
		pullRequest := event.GetPullRequest()
		prID := fmt.Sprintf("%d", pullRequest.GetNumber())

		return &common.PullRequestCommentMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  timestamp,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestID: prID,
		}
	case *github.PullRequestReviewThreadEvent:
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)
		pullRequest := event.GetPullRequest()
		prID := fmt.Sprintf("%d", pullRequest.GetNumber())

		return &common.PullRequestCommentMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestID: prID,
		}
	case *github.IssueCommentEvent:
		if !isPullRequest(event.GetIssue()) {
			return nil
		}

		comment := event.GetComment()
		timestamp := timestampToTime(comment.GetCreatedAt())
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)
		prID := fmt.Sprintf("%d", event.GetIssue().GetNumber())

		return &common.PullRequestCommentMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  timestamp,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetSender().GetLogin(),
			},
			PullRequestID: prID,
		}
	}
	return nil
}

func newPullRequestClosedMetrics(event interface{}, webhookType, appSlug string) *common.PullRequestClosedMetrics {
	switch event := event.(type) {
	case *github.PullRequestEvent:
		pullRequest := event.GetPullRequest()
		timestamp := timestampToTime(pullRequest.GetUpdatedAt())
		action := event.GetAction()
		originalTrigger := fmt.Sprintf("%s:%s", webhookType, action)

		pullRequestMetrics := newPullRequestMetrics(event.GetPullRequest())
		if pullRequest.GetMerged() {
			pullRequestMetrics.MergeCommitSHA = pullRequest.GetMergeCommitSHA()
		}

		return &common.PullRequestClosedMetrics{
			GeneralMetrics: common.GeneralMetrics{
				TimeStamp:       time.Now(),
				EventTimestamp:  timestamp,
				AppSlug:         appSlug,
				Action:          action,
				OriginalTrigger: originalTrigger,
				Username:        event.GetSender().GetLogin(),
				GitRef:          pullRequest.GetHead().GetRef(),
			},
			PullRequestMetrics: pullRequestMetrics,
			Status:             event.GetPullRequest().GetState(),
		}
	}
	return nil
}

func newPullRequestMetrics(pullRequest *github.PullRequest) common.PullRequestMetrics {
	prID := fmt.Sprintf("%d", pullRequest.GetNumber())

	return hookCommon.PullRequestMetrics{
		PullRequestID: prID,
		CommitID:      pullRequest.GetHead().GetSHA(),
		ChangedFiles:  pullRequest.GetChangedFiles(),
		Additions:     pullRequest.GetAdditions(),
		Deletions:     pullRequest.GetDeletions(),
		Commits:       pullRequest.GetCommits(),
	}
}

var pullRequestOpenedTriggers = map[string][]string{
	"pull_request": {
		"opened",
	},
}

func isPullRequestOpenedAction(event, action string) bool {
	supportedActions := pullRequestOpenedTriggers[event]
	return sliceutil.IsStringInSlice(action, supportedActions)
}

var pullRequestUpdatedTriggers = map[string][]string{
	"pull_request": {
		"reopened",
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
	"issue_comment": {
		"created",
		"edited",
		"deleted",
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
	"create": {
		"",
	},
	"delete": {
		"",
	},
}

func isPushAction(event, action string) bool {
	supportedActions := pushActions[event]
	return sliceutil.IsStringInSlice(action, supportedActions)
}

func timestampToTime(timestamp github.Timestamp) *time.Time {
	if !timestamp.Equal(github.Timestamp{}) {
		t := timestamp.GetTime()
		if !t.IsZero() {
			return t
		}
	}
	return nil
}

func oldestCommitTimestamp(commits []*github.HeadCommit) *time.Time {
	if len(commits) > 0 {
		return timestampToTime(commits[0].GetTimestamp())
	}
	return nil
}

func isPullRequest(issue *github.Issue) bool {
	return issue.GetPullRequestLinks() != nil
}
