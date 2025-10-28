package bitbucketserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

// TimestampFormat RFC3339 but timezone without colon
const TimestampFormat = "2006-01-02T15:04:05-0700"

// GatherMetrics ...
func (hp HookProvider) GatherMetrics(r *http.Request, appSlug string) ([]common.Metrics, error) {

	contentType, eventKey, err := detectContentTypeAndEventKey(r.Header)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(contentType, common.ContentTypeApplicationJSON) {
		return nil, fmt.Errorf("Content-Type is not supported: %s", contentType)
	}

	accepted := []string{"repo:refs_changed", "pr:opened", "pr:modified", "pr:merged", "pr:declined"}
	if !slices.Contains(accepted, eventKey) {
		return nil, nil
	}

	currentTime := hp.timeProvider.CurrentTime()

	if eventKey == "repo:refs_changed" {
		var pushEvent PushEventModel
		if err := json.NewDecoder(r.Body).Decode(&pushEvent); err != nil {
			return nil, fmt.Errorf("Failed to parse request body as JSON: %s", err)
		}

		return hp.gatherPushMetrics(pushEvent, eventKey, appSlug, currentTime)
	}
	if eventKey == "pr:opened" || eventKey == "pr:modified" || eventKey == "pr:merged" || eventKey == "pr:declined" {
		var pullRequestEvent PullRequestEventModel
		if err := json.NewDecoder(r.Body).Decode(&pullRequestEvent); err != nil {
			return nil, fmt.Errorf("Failed to parse request body as JSON: %s", err)
		}

		return hp.gatherPRMetrics(pullRequestEvent, eventKey, appSlug, currentTime)
	}

	return nil, nil
}

func (hp HookProvider) gatherPushMetrics(event PushEventModel, eventKey string, appSlug string, currentTime time.Time) ([]common.Metrics, error) {
	var metricsList []common.Metrics

	for _, change := range event.Changes {
		if change.Ref.Type == refTypeTag {
			continue
		}

		var constructorFunc func(generalMetrics common.GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) common.PushMetrics
		// general metrics
		provider := ProviderID
		repo := event.RepositoryInfo.Project.Key + "/" + event.RepositoryInfo.Slug
		timestamp, err := time.Parse(TimestampFormat, event.Date)
		if err != nil {
			return nil, err
		}
		originalTrigger := common.OriginalTrigger(eventKey, "")
		userName := event.Actor.Name
		gitRef := change.Ref.DisplayID
		// push metrics
		commitIDAfter := change.ToHash
		commitIDBefore := change.FromHash
		var oldestCommitTime *time.Time
		var latestCommitTime *time.Time
		var masterBranch string

		isBranchCreated := change.FromHash == ""
		isBranchDeleted := change.ToHash == ""

		switch {
		case isBranchCreated:
			constructorFunc = common.NewPushCreatedMetrics
		case isBranchDeleted:
			constructorFunc = common.NewPushDeletedMetrics
		default:
			constructorFunc = common.NewPushMetrics
		}

		generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, &timestamp, appSlug, originalTrigger, userName, gitRef)
		metrics := constructorFunc(generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTime, latestCommitTime, masterBranch)
		metricsList = append(metricsList, metrics)
	}

	return metricsList, nil
}

func (hp HookProvider) gatherPRMetrics(event PullRequestEventModel, eventKey string, appSlug string, currentTime time.Time) ([]common.Metrics, error) {
	var constructorFunc func(generalMetrics common.GeneralMetrics, generalPullRequestMetrics common.GeneralPullRequestMetrics) common.PullRequestMetrics

	provider := ProviderID
	timestamp, err := time.Parse(TimestampFormat, event.Date)
	if err != nil {
		return nil, err
	}
	originalTrigger := common.OriginalTrigger(eventKey, "")
	userName := event.Actor.Name

	var pullRequest PullRequestInfoModel
	pullRequest = event.PullRequest
	repo := pullRequest.ToRef.Repository.Project.Key + "/" + event.PullRequest.ToRef.Repository.Slug
	gitRef := pullRequest.FromRef.DisplayID

	switch eventKey {
	case "pr:opened":
		constructorFunc = common.NewPullRequestOpenedMetrics
	case "pr:modified":
		constructorFunc = common.NewPullRequestUpdatedMetrics
	case "pr:merged":
		constructorFunc = common.NewPullRequestClosedMetrics
	case "pr:declined":
		constructorFunc = common.NewPullRequestClosedMetrics
	default:
		return nil, nil
	}

	generalMetrics := common.NewGeneralMetrics(provider, repo, currentTime, &timestamp, appSlug, originalTrigger, userName, gitRef)
	generalPullRequestMetrics := newGeneralPullRequestMetrics(pullRequest)
	metrics := constructorFunc(generalMetrics, generalPullRequestMetrics)
	return []common.Metrics{metrics}, nil
}

func newGeneralPullRequestMetrics(pullRequest PullRequestInfoModel) common.GeneralPullRequestMetrics {
	prID := fmt.Sprintf("%d", pullRequest.ID)

	status := strings.ToLower(pullRequest.State) // OPEN, MERGED or DECLINED
	if status == "open" {
		status = "opened"
	}

	return common.GeneralPullRequestMetrics{
		PullRequestTitle: pullRequest.Title,
		PullRequestID:    prID,
		TargetBranch:     pullRequest.ToRef.DisplayID,
		CommitID:         pullRequest.FromRef.LatestCommit,
		Status:           status,
	}
}
