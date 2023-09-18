package github

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
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
	var constructorFunc func(generalMetrics common.GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) common.PushMetrics
	// general metrics
	var timestamp *time.Time
	var originalTrigger string
	var userName string
	var gitRef string
	// push metrics
	var commitIDAfter string
	var commitIDBefore string
	var oldestCommitTime *time.Time
	var latestCommitTime *time.Time
	var masterBranch string

	switch event := event.(type) {
	case *github.PushEvent:
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

		timestamp = timestampToTime(event.GetHeadCommit().GetTimestamp())
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, event.GetAction())
		userName = event.GetPusher().GetName()
		gitRef = event.GetRef()

		commitIDAfter = event.GetAfter()
		commitIDBefore = event.GetBefore()
		oldestCommitTime = oldestCommitTimestamp(event.GetCommits())
		latestCommitTime = latestCommitTimestamp(event.GetCommits())
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
	metrics := constructorFunc(generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTime, latestCommitTime, masterBranch)
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
		pullRequest = event.GetPullRequest()
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
		userName = pullRequest.GetUser().GetLogin()
		gitRef = pullRequest.GetHead().GetRef()

		if isPullRequestOpenedAction(action) {
			constructorFunc = common.NewPullRequestOpenedMetrics
			timestamp = timestampToTime(pullRequest.GetCreatedAt())
			mergeCommitSHA = ""
		} else if isPullRequestClosedAction(action) {
			constructorFunc = common.NewPullRequestClosedMetrics
			timestamp = timestampToTime(pullRequest.GetUpdatedAt())
			if pullRequest.GetMerged() {
				mergeCommitSHA = pullRequest.GetMergeCommitSHA()
			}
		} else { // Pull request updated
			constructorFunc = common.NewPullRequestUpdatedMetrics
			timestamp = timestampToTime(pullRequest.GetUpdatedAt())
			mergeCommitSHA = ""
		}
	case *github.PullRequestReviewEvent:
		action := event.GetAction()
		constructorFunc = common.NewPullRequestUpdatedMetrics
		pullRequest = event.GetPullRequest()
		timestamp = timestampToTime(pullRequest.GetUpdatedAt())
		originalTrigger = fmt.Sprintf("%s:%s", webhookType, action)
		userName = pullRequest.GetUser().GetLogin()
		gitRef = pullRequest.GetHead().GetRef()
		mergeCommitSHA = ""
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

		timestamp = timestampToTime(comment.GetUpdatedAt())
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

		timestamp = timestampToTime(comment.GetUpdatedAt())
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

func isPullRequestOpenedAction(action string) bool {
	return action == "opened"
}

func isPullRequestClosedAction(action string) bool {
	return action == "closed"
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

func latestCommitTimestamp(commits []*github.HeadCommit) *time.Time {
	if len(commits) > 0 {
		return timestampToTime(commits[len(commits)-1].GetTimestamp())
	}
	return nil
}

func isPullRequest(issue *github.Issue) bool {
	return issue.GetPullRequestLinks() != nil
}
