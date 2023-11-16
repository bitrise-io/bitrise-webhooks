package bitbucketv2

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/go-playground/webhooks/v6/bitbucket"
)

// GatherMetrics ...
func (hp HookProvider) GatherMetrics(r *http.Request, appSlug string) ([]common.Metrics, error) {
	hook, err := bitbucket.New()
	if err != nil {
		return nil, err
	}

	payload, err := hook.Parse(r, bitbucket.RepoPushEvent, bitbucket.PullRequestCreatedEvent, bitbucket.PullRequestUpdatedEvent, bitbucket.PullRequestMergedEvent, bitbucket.PullRequestDeclinedEvent)
	if err != nil {
		if err == bitbucket.ErrEventNotFound {
			return nil, nil
		}
		return nil, err
	}

	event := r.Header.Get("X-Event-Key")
	currentTime := hp.timeProvider.CurrentTime()

	metricsList := hp.gatherMetrics(payload, event, appSlug, currentTime)
	return metricsList, nil
}

func (hp HookProvider) gatherMetrics(payload interface{}, webhookType, appSlug string, currentTime time.Time) []common.Metrics {
	switch payload := payload.(type) {
	case bitbucket.RepoPushPayload:
		return newPushMetrics(payload, webhookType, appSlug, currentTime)
	case bitbucket.PullRequestCreatedPayload, bitbucket.PullRequestUpdatedPayload, bitbucket.PullRequestMergedPayload, bitbucket.PullRequestDeclinedPayload:
		return newPullRequestMetrics(payload, webhookType, appSlug, currentTime)
	}

	return nil
}

func newPushMetrics(payload bitbucket.RepoPushPayload, webhookType, appSlug string, currentTime time.Time) []common.Metrics {
	var metricsList []common.Metrics

	for _, change := range payload.Push.Changes {
		if change.New.Target.Type == "tag" || change.Old.Target.Type == "tag" {
			continue
		}

		var constructorFunc func(generalMetrics common.GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) common.PushMetrics
		// general metrics
		provider := ProviderID
		repo := payload.Repository.FullName
		var timestamp *time.Time
		originalTrigger := common.OriginalTrigger(webhookType, "")
		userName := payload.Actor.NickName
		var gitRef string
		// push metrics
		commitIDAfter := change.New.Target.Hash
		commitIDBefore := change.Old.Target.Hash
		var oldestCommitTime *time.Time
		var latestCommitTime *time.Time
		var masterBranch string

		isBranchCreated := change.Old.Target.Hash == ""
		isBranchDeleted := change.New.Target.Hash == ""
		isForcePush := change.Forced

		switch {
		case isBranchCreated:
			constructorFunc = common.NewPushCreatedMetrics
			timestamp = &change.New.Target.Date
			gitRef = change.New.Name
		case isBranchDeleted:
			constructorFunc = common.NewPushDeletedMetrics
			gitRef = change.Old.Name
		case isForcePush:
			constructorFunc = common.NewPushForcedMetrics
			timestamp = &change.New.Target.Date
			gitRef = change.New.Name
		default:
			constructorFunc = common.NewPushMetrics
			timestamp = &change.New.Target.Date
			gitRef = change.New.Name
		}

		generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, timestamp, appSlug, originalTrigger, userName, gitRef)
		metrics := constructorFunc(generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTime, latestCommitTime, masterBranch)
		metricsList = append(metricsList, metrics)
	}

	return metricsList
}

func newPullRequestMetrics(payload interface{}, webhookType, appSlug string, currentTime time.Time) []common.Metrics {
	var constructorFunc func(generalMetrics common.GeneralMetrics, generalPullRequestMetrics common.GeneralPullRequestMetrics) common.PullRequestMetrics
	// general metrics
	provider := ProviderID
	var repo string
	var timestamp *time.Time
	originalTrigger := common.OriginalTrigger(webhookType, "")
	var userName string
	var gitRef string
	// pull request metrics
	var pullRequest bitbucket.PullRequest

	switch payload := payload.(type) {
	case bitbucket.PullRequestCreatedPayload:
		constructorFunc = common.NewPullRequestOpenedMetrics
		pullRequest = payload.PullRequest
		repo = payload.Repository.FullName
		timestamp = &pullRequest.CreatedOn
		userName = payload.Actor.NickName
		gitRef = pullRequest.Source.Branch.Name
	case bitbucket.PullRequestUpdatedPayload:
		constructorFunc = common.NewPullRequestUpdatedMetrics
		pullRequest = payload.PullRequest
		repo = payload.Repository.FullName
		timestamp = &pullRequest.UpdatedOn
		userName = payload.Actor.NickName
		gitRef = pullRequest.Source.Branch.Name
	case bitbucket.PullRequestMergedPayload:
		constructorFunc = common.NewPullRequestClosedMetrics
		pullRequest = payload.PullRequest
		repo = payload.Repository.FullName
		timestamp = &pullRequest.UpdatedOn
		userName = payload.Actor.NickName
		gitRef = pullRequest.Source.Branch.Name
	case bitbucket.PullRequestDeclinedPayload:
		constructorFunc = common.NewPullRequestClosedMetrics
		pullRequest = payload.PullRequest
		repo = payload.Repository.FullName
		timestamp = &pullRequest.UpdatedOn
		userName = payload.Actor.NickName
		gitRef = pullRequest.Source.Branch.Name
	default:
		return nil
	}

	generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, timestamp, appSlug, originalTrigger, userName, gitRef)
	generalPullRequestMetrics := newGeneralPullRequestMetrics(pullRequest)
	metrics := constructorFunc(generalMetrics, generalPullRequestMetrics)
	return []common.Metrics{metrics}
}

func newGeneralPullRequestMetrics(pullRequest bitbucket.PullRequest) common.GeneralPullRequestMetrics {
	prID := fmt.Sprintf("%d", pullRequest.ID)

	status := strings.ToLower(pullRequest.State) // OPEN, MERGED or DECLINED
	if status == "open" {
		status = "opened"
	}

	return common.GeneralPullRequestMetrics{
		PullRequestTitle: pullRequest.Title,
		PullRequestID:    prID,
		PullRequestURL:   pullRequest.Links.HTML.Href,
		TargetBranch:     pullRequest.Destination.Branch.Name,
		CommitID:         pullRequest.Source.Commit.Hash,
		MergeCommitSHA:   pullRequest.MergeCommit.Hash,
		Status:           status,
	}
}
