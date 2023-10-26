package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Event ...
type Event string

const (
	// PushEvent ...
	PushEvent Event = "git_push"
	// PullRequestEvent ...
	PullRequestEvent Event = "pull_request"
)

// Action ...
type Action string

const (
	// PushPushedAction represents a push event.
	PushPushedAction Action = "pushed"
	// PushForcedAction represents a force push event.
	PushForcedAction Action = "forced"
	// PushCreatedAction represents a push event which created a ref.
	PushCreatedAction Action = "created"
	// PushDeletedAction represents a push event which deleted a ref.
	PushDeletedAction Action = "deleted"

	// PullRequestOpenedAction ...
	PullRequestOpenedAction Action = "opened"
	// PullRequestUpdatedAction ...
	PullRequestUpdatedAction Action = "updated"
	// PullRequestClosedAction ...
	PullRequestClosedAction Action = "closed"
	// PullRequestCommentAction ...
	PullRequestCommentAction Action = "comment"
)

// Metrics ...
type Metrics interface {
	Serialise() ([]byte, error)
}

// MetricsProvider ...
type MetricsProvider interface {
	GatherMetrics(r *http.Request, appSlug string) (Metrics, error)
}

// PushMetrics ...
type PushMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action Action `json:"action,omitempty"`

	GeneralMetrics

	CommitIDAfter         string     `json:"commit_id_after,omitempty"`
	CommitIDBefore        string     `json:"commit_id_before,omitempty"`
	OldestCommitTimestamp *time.Time `json:"oldest_commit_timestamp,omitempty"`
	LatestCommitTimestamp *time.Time `json:"latest_commit_timestamp,omitempty"`
	MasterBranch          string     `json:"master_branch,omitempty"`
}

// NewPushCreatedMetrics ...
func NewPushCreatedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics(PushCreatedAction, generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, latestCommitTimestamp, masterBranch)
}

// NewPushDeletedMetrics ...
func NewPushDeletedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics(PushDeletedAction, generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, latestCommitTimestamp, masterBranch)
}

// NewPushForcedMetrics ...
func NewPushForcedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics(PushForcedAction, generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, latestCommitTimestamp, masterBranch)
}

// NewPushMetrics ...
func NewPushMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics(PushPushedAction, generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, latestCommitTimestamp, masterBranch)
}

func newPushMetrics(action Action, generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, latestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return PushMetrics{
		Event:                 PushEvent,
		Action:                action,
		GeneralMetrics:        generalMetrics,
		CommitIDAfter:         commitIDAfter,
		CommitIDBefore:        commitIDBefore,
		OldestCommitTimestamp: oldestCommitTimestamp,
		LatestCommitTimestamp: latestCommitTimestamp,
		MasterBranch:          masterBranch,
	}
}

// PullRequestMetrics ...
type PullRequestMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action Action `json:"action,omitempty"`

	GeneralMetrics
	GeneralPullRequestMetrics
}

// NewPullRequestOpenedMetrics ...
func NewPullRequestOpenedMetrics(generalMetrics GeneralMetrics, generalPullRequestMetrics GeneralPullRequestMetrics) PullRequestMetrics {
	return newPullRequestMetrics(PullRequestOpenedAction, generalMetrics, generalPullRequestMetrics)
}

// NewPullRequestUpdatedMetrics ...
func NewPullRequestUpdatedMetrics(generalMetrics GeneralMetrics, generalPullRequestMetrics GeneralPullRequestMetrics) PullRequestMetrics {
	return newPullRequestMetrics(PullRequestUpdatedAction, generalMetrics, generalPullRequestMetrics)
}

// NewPullRequestClosedMetrics ...
func NewPullRequestClosedMetrics(generalMetrics GeneralMetrics, generalPullRequestMetrics GeneralPullRequestMetrics) PullRequestMetrics {
	return newPullRequestMetrics(PullRequestClosedAction, generalMetrics, generalPullRequestMetrics)
}

func newPullRequestMetrics(action Action, generalMetrics GeneralMetrics, generalPullRequestMetrics GeneralPullRequestMetrics) PullRequestMetrics {
	return PullRequestMetrics{
		Event:                     PullRequestEvent,
		Action:                    action,
		GeneralMetrics:            generalMetrics,
		GeneralPullRequestMetrics: generalPullRequestMetrics,
	}
}

// PullRequestCommentMetrics ...
type PullRequestCommentMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action Action `json:"action,omitempty"`

	GeneralMetrics
	PullRequestID string `json:"pull_request_id,omitempty"` // PR number
}

// NewPullRequestCommentMetrics ...
func NewPullRequestCommentMetrics(generalMetrics GeneralMetrics, pullRequestID string) PullRequestCommentMetrics {
	return PullRequestCommentMetrics{
		Event:          PullRequestEvent,
		Action:         PullRequestCommentAction,
		GeneralMetrics: generalMetrics,
		PullRequestID:  pullRequestID}
}

// GeneralMetrics ...
type GeneralMetrics struct {
	ProviderType    string     `json:"provider_type,omitempty"`
	Repository      string     `json:"repository,omitempty"` // org/repo
	TimeStamp       time.Time  `json:"timestamp,omitempty"`
	EventTimestamp  *time.Time `json:"event_timestamp,omitempty"`
	AppSlug         string     `json:"app_slug,omitempty"`
	OriginalTrigger string     `json:"original_trigger,omitempty"`
	Username        string     `json:"user_name,omitempty"`
	GitRef          string     `json:"git_ref,omitempty"`
}

// NewGeneralMetrics ...
func NewGeneralMetrics(providerType string, repository string, currentTime time.Time, eventTimestamp *time.Time, appSlug string, originalTrigger string, username string, gitRef string) GeneralMetrics {
	return GeneralMetrics{
		ProviderType:    providerType,
		Repository:      repository,
		TimeStamp:       currentTime,
		EventTimestamp:  eventTimestamp,
		AppSlug:         appSlug,
		OriginalTrigger: originalTrigger,
		Username:        username,
		GitRef:          gitRef,
	}
}

// GeneralPullRequestMetrics ...
type GeneralPullRequestMetrics struct {
	PullRequestTitle string `json:"pull_request_title,omitempty"`
	PullRequestID    string `json:"pull_request_id,omitempty"` // PR number
	PullRequestURL   string `json:"pull_request_url,omitempty"`
	TargetBranch     string `json:"target_branch,omitempty"`
	CommitID         string `json:"commit_id,omitempty"`
	ChangedFiles     int    `json:"changed_files_count"`
	Additions        int    `json:"addition_count"`
	Deletions        int    `json:"deletion_count"`
	Commits          int    `json:"commit_count"`
	MergeCommitSHA   string `json:"merge_commit_sha,omitempty"`
	Status           string `json:"status,omitempty"`
}

// Serialise ...
func (m PushMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

// Serialise ...
func (m PullRequestMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

// Serialise ...
func (m PullRequestCommentMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

// String ...
func (m PushMetrics) String() string {
	return stringer(m)
}

// String ...
func (m PullRequestMetrics) String() string {
	return stringer(m)
}

// String ...
func (m PullRequestCommentMetrics) String() string {
	return stringer(m)
}

func stringer(v interface{}) string {
	c, err := json.MarshalIndent(v, "", "\t")
	if err == nil {
		return string(c)
	}
	return fmt.Sprintf("#%v", v)
}
