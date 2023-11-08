package github

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/google/go-github/v55/github"
)

// GatherMetrics ...
func (hp HookProvider) GatherMetrics(r *http.Request, appSlug string) ([]common.Metrics, error) {
	payload, err := github.ValidatePayload(r, nil)
	if err != nil {
		return nil, err
	}

	webhookType := github.WebHookType(r)

	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		return nil, err
	}

	currentTime := hp.timeProvider.CurrentTime()
	metrics, err := hp.gatherMetrics(event, webhookType, appSlug, currentTime), nil
	if err != nil {
		return nil, err
	}

	return []common.Metrics{metrics}, nil
}

func (hp HookProvider) gatherMetrics(event interface{}, webhookType, appSlug string, currentTime time.Time) common.Metrics {
	var metrics common.Metrics
	switch event := event.(type) {
	case *github.PushEvent, *github.DeleteEvent, *github.CreateEvent:
		metrics = newPushMetrics(event, webhookType, appSlug, currentTime)
	case *github.PullRequestEvent, *github.PullRequestReviewEvent:
		metrics = newPullRequestMetrics(event, webhookType, appSlug, currentTime)
	case *github.PullRequestReviewCommentEvent, *github.PullRequestReviewThreadEvent, *github.IssueCommentEvent:
		metrics = newPullRequestCommentMetrics(event, webhookType, appSlug, currentTime)
	}

	return metrics
}

func newPushMetrics(event interface{}, webhookType, appSlug string, currentTime time.Time) *common.PushMetrics {
	var constructorFunc func(generalMetrics common.GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) common.PushMetrics
	// general metrics
	provider := ProviderID
	var repo string
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

		repo = event.GetRepo().GetFullName()
		timestamp = timestampToTime(event.GetHeadCommit().GetTimestamp())
		originalTrigger = common.OriginalTrigger(webhookType, event.GetAction())
		userName = event.GetPusher().GetName()
		gitRef = event.GetRef()

		commitIDAfter = event.GetAfter()
		commitIDBefore = event.GetBefore()
		oldestCommitTime = oldestCommitTimestamp(event.GetCommits())
		latestCommitTime = latestCommitTimestamp(event.GetCommits())
		masterBranch = event.GetRepo().GetDefaultBranch()
	case *github.DeleteEvent:
		constructorFunc = common.NewPushDeletedMetrics

		repo = event.GetRepo().GetFullName()
		timestamp = nil
		originalTrigger = common.OriginalTrigger(webhookType, "")
		userName = event.GetSender().GetLogin()
		gitRef = event.GetRef()

		commitIDAfter = ""
		commitIDBefore = ""
		oldestCommitTime = nil
		masterBranch = event.GetRepo().GetDefaultBranch()
	case *github.CreateEvent:
		constructorFunc = common.NewPushCreatedMetrics

		repo = event.GetRepo().GetFullName()
		timestamp = nil
		originalTrigger = common.OriginalTrigger(webhookType, "")
		userName = event.GetSender().GetLogin()
		gitRef = event.GetRef()

		commitIDAfter = ""
		commitIDBefore = ""
		oldestCommitTime = nil
		masterBranch = event.GetRepo().GetDefaultBranch()
	default:
		return nil
	}

	generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, timestamp, appSlug, originalTrigger, userName, gitRef)
	metrics := constructorFunc(generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTime, latestCommitTime, masterBranch)
	return &metrics
}

func newPullRequestMetrics(event interface{}, webhookType, appSlug string, currentTime time.Time) *common.PullRequestMetrics {
	var constructorFunc func(generalMetrics common.GeneralMetrics, generalPullRequestMetrics common.GeneralPullRequestMetrics) common.PullRequestMetrics
	// general metrics
	provider := ProviderID
	var repo string
	var timestamp *time.Time
	var originalTrigger string
	var userName string
	var gitRef string
	// pull request metrics
	var pullRequest *github.PullRequest
	var mergeCommitSHA string

	switch event := event.(type) {
	case *github.PullRequestEvent:
		repo = event.GetRepo().GetFullName()
		action := event.GetAction()
		pullRequest = event.GetPullRequest()
		originalTrigger = common.OriginalTrigger(webhookType, action)
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
		repo = event.GetRepo().GetFullName()
		action := event.GetAction()
		constructorFunc = common.NewPullRequestUpdatedMetrics
		pullRequest = event.GetPullRequest()
		timestamp = timestampToTime(pullRequest.GetUpdatedAt())
		originalTrigger = common.OriginalTrigger(webhookType, action)
		userName = pullRequest.GetUser().GetLogin()
		gitRef = pullRequest.GetHead().GetRef()
		mergeCommitSHA = ""
	default:
		return nil
	}

	generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, timestamp, appSlug, originalTrigger, userName, gitRef)
	generalPullRequestMetrics := newGeneralPullRequestMetrics(pullRequest, mergeCommitSHA)
	metrics := constructorFunc(generalMetrics, generalPullRequestMetrics)
	return &metrics

}

func newPullRequestCommentMetrics(event interface{}, webhookType, appSlug string, currentTime time.Time) *common.PullRequestCommentMetrics {
	// general metrics
	provider := ProviderID
	var repo string
	var timestamp *time.Time
	var originalTrigger string
	var userName string
	var gitRef string
	// pull request metrics
	var prID string

	switch event := event.(type) {
	case *github.PullRequestReviewCommentEvent:
		repo = event.GetRepo().GetFullName()
		comment := event.GetComment()
		action := event.GetAction()
		pullRequest := event.GetPullRequest()

		timestamp = timestampToTime(comment.GetUpdatedAt())
		originalTrigger = common.OriginalTrigger(webhookType, action)
		userName = event.GetSender().GetLogin()
		gitRef = pullRequest.GetHead().GetRef()
		prID = fmt.Sprintf("%d", pullRequest.GetNumber())
	case *github.PullRequestReviewThreadEvent:
		repo = event.GetRepo().GetFullName()
		action := event.GetAction()
		pullRequest := event.GetPullRequest()

		timestamp = nil
		originalTrigger = common.OriginalTrigger(webhookType, action)
		userName = event.GetSender().GetLogin()
		gitRef = pullRequest.GetHead().GetRef()
		prID = fmt.Sprintf("%d", pullRequest.GetNumber())
	case *github.IssueCommentEvent:
		if !isPullRequest(event.GetIssue()) {
			return nil
		}

		repo = event.GetRepo().GetFullName()
		comment := event.GetComment()
		action := event.GetAction()

		timestamp = timestampToTime(comment.GetUpdatedAt())
		originalTrigger = common.OriginalTrigger(webhookType, action)
		userName = event.GetSender().GetLogin()
		gitRef = ""
		prID = fmt.Sprintf("%d", event.GetIssue().GetNumber())
	default:
		return nil
	}

	generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, timestamp, appSlug, originalTrigger, userName, gitRef)
	metrics := common.NewPullRequestCommentMetrics(generalMetrics, prID)
	return &metrics
}

func newGeneralPullRequestMetrics(pullRequest *github.PullRequest, mergeCommitSHA string) common.GeneralPullRequestMetrics {
	prID := fmt.Sprintf("%d", pullRequest.GetNumber())
	status := pullRequest.GetState()
	if status == "open" {
		status = "opened"
	}

	return common.GeneralPullRequestMetrics{
		PullRequestTitle: pullRequest.GetTitle(),
		PullRequestID:    prID,
		PullRequestURL:   pullRequest.GetHTMLURL(),
		TargetBranch:     pullRequest.GetBase().GetRef(),
		CommitID:         pullRequest.GetHead().GetSHA(),
		ChangedFiles:     pullRequest.GetChangedFiles(),
		Additions:        pullRequest.GetAdditions(),
		Deletions:        pullRequest.GetDeletions(),
		Commits:          pullRequest.GetCommits(),
		MergeCommitSHA:   mergeCommitSHA,
		Status:           status,
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
