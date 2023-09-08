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
func (hp HookProvider) GatherMetrics(r *http.Request, appSlug string) (common.Metrics, error) {
	payload, err := github.ValidatePayload(r, nil)
	if err != nil {
		return nil, err
	}

	webhookType := github.WebHookType(r)

	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		return nil, err
	}

	var metrics common.Metrics
	switch event := event.(type) {
	case *github.PushEvent, *github.DeleteEvent, *github.CreateEvent:
		metrics = newPushMetrics(event, webhookType, appSlug)
	case *github.PullRequestEvent, *github.PullRequestReviewEvent:
		metrics = newPullRequestMetrics(event, webhookType, appSlug)
	case *github.PullRequestReviewCommentEvent, *github.PullRequestReviewThreadEvent, *github.IssueCommentEvent:
		metrics = newPullRequestCommentMetrics(event, webhookType, appSlug)
	}

	return metrics, nil
}

func newPushMetrics(event interface{}, webhookType, appSlug string) *common.PushMetrics {
	var constructorFunc func(generalMetrics common.GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) common.PushMetrics
	// general metrics
	var timestamp *time.Time
	var originalTrigger string
	var userName string
	var gitRef string
	// push metrics
	var commitIDAfter string
	var commitIDBefore string
	var oldestCommitTime *time.Time
	var masterBranch string

	switch event := event.(type) {
	case *github.PushEvent:
		switch webhookType {
		case "push":
			switch {
			case event.GetCreated():
				constructorFunc = common.NewPushCreatedMetrics
			case event.GetDeleted():
				constructorFunc = common.NewPushDeletedMetrics
			case event.GetForced():
				constructorFunc = common.NewPushForcedMetrics
			default:
				constructorFunc = common.NewPushMetrics
			}
		}

		timestamp = timestampToTime(event.GetHeadCommit().GetTimestamp())
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, event.GetAction())
		userName = event.GetPusher().GetName()
		gitRef = event.GetRef()
		commitIDAfter = event.GetAfter()
		commitIDBefore = event.GetBefore()
		oldestCommitTime = oldestCommitTimestamp(event.GetCommits())
		masterBranch = ""
	case *github.DeleteEvent:
		constructorFunc = common.NewPushDeletedMetrics
		timestamp = nil
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, "")
		userName = event.GetSender().GetLogin()
		gitRef = event.GetRef()
		commitIDAfter = ""
		commitIDBefore = ""
		oldestCommitTime = nil
		masterBranch = ""
	case *github.CreateEvent:
		constructorFunc = common.NewPushCreatedMetrics
		timestamp = nil
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, "")
		userName = event.GetSender().GetLogin()
		gitRef = event.GetRef()
		commitIDAfter = ""
		commitIDBefore = ""
		oldestCommitTime = nil
		masterBranch = event.GetMasterBranch()
	default:
		return nil
	}

	generalMetrics := common.NewGeneralMetrics(timestamp, appSlug, originalTrigger, userName, gitRef)
	metrics := constructorFunc(generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTime, masterBranch)
	return &metrics
}

func newPullRequestMetrics(event interface{}, webhookType, appSlug string) *common.PullRequestMetrics {
	var constructorFunc func(generalMetrics common.GeneralMetrics, generalPullRequestMetrics common.GeneralPullRequestMetrics) common.PullRequestMetrics
	// general metrics
	var timestamp *time.Time
	var originalTrigger string
	var userName string
	var gitRef string
	// pull request metrics
	var pullRequest *github.PullRequest
	var mergeCommitSHA string

	switch event := event.(type) {
	case *github.PullRequestEvent:
		action := event.GetAction()
		if isPullRequestOpenedAction(webhookType, action) {
			constructorFunc = common.NewPullRequestOpenedMetrics
			pullRequest = event.GetPullRequest()
			timestamp = timestampToTime(pullRequest.GetCreatedAt())
			originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
			userName = pullRequest.GetUser().GetLogin()
			gitRef = pullRequest.GetHead().GetRef()
			mergeCommitSHA = ""
		} else if isPullRequestUpdatedAction(webhookType, action) {
			constructorFunc = common.NewPullRequestUpdatedMetrics
			pullRequest = event.GetPullRequest()
			timestamp = timestampToTime(pullRequest.GetUpdatedAt())
			originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
			userName = pullRequest.GetUser().GetLogin()
			gitRef = pullRequest.GetHead().GetRef()
			mergeCommitSHA = ""
		} else if isPullRequestClosedAction(webhookType, action) {
			constructorFunc = common.NewPullRequestClosedMetrics
			pullRequest = event.GetPullRequest()
			timestamp = timestampToTime(pullRequest.GetUpdatedAt())
			originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
			userName = pullRequest.GetUser().GetLogin()
			gitRef = pullRequest.GetHead().GetRef()
			mergeCommitSHA = ""
		} else {
			return nil
		}
	case *github.PullRequestReviewEvent:
		action := event.GetAction()
		if isPullRequestUpdatedAction(webhookType, action) {
			constructorFunc = common.NewPullRequestUpdatedMetrics
			pullRequest = event.GetPullRequest()
			timestamp = timestampToTime(pullRequest.GetUpdatedAt())
			originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
			userName = pullRequest.GetUser().GetLogin()
			gitRef = pullRequest.GetHead().GetRef()
			if pullRequest.GetMerged() {
				mergeCommitSHA = pullRequest.GetMergeCommitSHA()
			}
		} else {
			return nil
		}
	default:
		return nil
	}

	generalMetrics := common.NewGeneralMetrics(timestamp, appSlug, originalTrigger, userName, gitRef)
	generalPullRequestMetrics := newGeneralPullRequestMetrics(pullRequest, mergeCommitSHA)
	metrics := constructorFunc(generalMetrics, generalPullRequestMetrics)
	return &metrics

}

func newPullRequestCommentMetrics(event interface{}, webhookType, appSlug string) *common.PullRequestCommentMetrics {
	// general metrics
	var timestamp *time.Time
	var originalTrigger string
	var userName string
	var gitRef string
	// pull request metrics
	var prID string

	switch event := event.(type) {
	case *github.PullRequestReviewCommentEvent:
		comment := event.GetComment()
		action := event.GetAction()
		pullRequest := event.GetPullRequest()

		timestamp = timestampToTime(comment.GetCreatedAt())
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
		userName = event.GetSender().GetLogin()
		gitRef = pullRequest.GetHead().GetRef()
		prID = fmt.Sprintf("%d", pullRequest.GetNumber())
	case *github.PullRequestReviewThreadEvent:
		action := event.GetAction()
		pullRequest := event.GetPullRequest()

		timestamp = nil
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
		userName = event.GetSender().GetLogin()
		gitRef = pullRequest.GetHead().GetRef()
		prID = fmt.Sprintf("%d", pullRequest.GetNumber())
	case *github.IssueCommentEvent:
		if !isPullRequest(event.GetIssue()) {
			return nil
		}

		comment := event.GetComment()
		action := event.GetAction()

		timestamp = timestampToTime(comment.GetCreatedAt())
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
		userName = event.GetSender().GetLogin()
		gitRef = ""
		prID = fmt.Sprintf("%d", event.GetIssue().GetNumber())
	default:
		return nil
	}

	generalMetrics := common.NewGeneralMetrics(timestamp, appSlug, originalTrigger, userName, gitRef)
	metrics := common.NewPullRequestCommentMetrics(generalMetrics, prID)
	return &metrics
}

func newGeneralPullRequestMetrics(pullRequest *github.PullRequest, mergeCommitSHA string) common.GeneralPullRequestMetrics {
	prID := fmt.Sprintf("%d", pullRequest.GetNumber())

	return hookCommon.GeneralPullRequestMetrics{
		PullRequestID:  prID,
		CommitID:       pullRequest.GetHead().GetSHA(),
		ChangedFiles:   pullRequest.GetChangedFiles(),
		Additions:      pullRequest.GetAdditions(),
		Deletions:      pullRequest.GetDeletions(),
		Commits:        pullRequest.GetCommits(),
		MergeCommitSHA: mergeCommitSHA,
		Status:         pullRequest.GetState(),
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
