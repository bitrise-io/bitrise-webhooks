package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Event string

const (
	PushEvent        Event = "git_push"
	PullRequestEvent Event = "pull_request"
)

type Action string

const (
	PushCreatedAction        Action = "created"
	PushDeletedAction        Action = "deleted"
	PushForcedAction         Action = "forced"
	PushPushedAction         Action = "pushed"
	PullRequestOpenedAction  Action = "opened"
	PullRequestUpdatedAction Action = "updated"
	PullRequestClosedAction  Action = "closed"
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

	CommitIDAfter         string     `json:"commit_id_before,omitempty"`
	CommitIDBefore        string     `json:"commit_id_after,omitempty"`
	OldestCommitTimestamp *time.Time `json:"oldest_commit_timestamp,omitempty"`
	MasterBranch          string     `json:"master_branch,omitempty"`
}

func NewPushCreatedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics(PushCreatedAction, generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, masterBranch)
}

func NewPushDeletedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics(PushDeletedAction, generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, masterBranch)
}

func NewPushForcedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics(PushForcedAction, generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, masterBranch)
}

func NewPushMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics(PushPushedAction, generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, masterBranch)
}

func newPushMetrics(action Action, generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return PushMetrics{
		Event:                 PushEvent,
		Action:                action,
		GeneralMetrics:        generalMetrics,
		CommitIDAfter:         commitIDAfter,
		CommitIDBefore:        commitIDBefore,
		OldestCommitTimestamp: oldestCommitTimestamp,
		MasterBranch:          masterBranch,
	}
}

type PullRequestMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action Action `json:"action,omitempty"`

	GeneralMetrics
	GeneralPullRequestMetrics
}

func NewPullRequestOpenedMetrics(generalMetrics GeneralMetrics, generalPullRequestMetrics GeneralPullRequestMetrics) PullRequestMetrics {
	return newPullRequestMetrics(PullRequestOpenedAction, generalMetrics, generalPullRequestMetrics)
}

func NewPullRequestUpdatedMetrics(generalMetrics GeneralMetrics, generalPullRequestMetrics GeneralPullRequestMetrics) PullRequestMetrics {
	return newPullRequestMetrics(PullRequestUpdatedAction, generalMetrics, generalPullRequestMetrics)
}

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

func NewPullRequestCommentMetrics(generalMetrics GeneralMetrics, pullRequestID string) PullRequestCommentMetrics {
	return PullRequestCommentMetrics{
		Event:          PullRequestEvent,
		Action:         PullRequestCommentAction,
		GeneralMetrics: generalMetrics,
		PullRequestID:  pullRequestID}
}

// GeneralMetrics ...
type GeneralMetrics struct {
	TimeStamp       time.Time  `json:"timestamp,omitempty"`
	EventTimestamp  *time.Time `json:"event_timestamp,omitempty"`
	AppSlug         string     `json:"app_slug,omitempty"`
	OriginalTrigger string     `json:"original_trigger,omitempty"`
	Username        string     `json:"user_name,omitempty"`
	GitRef          string     `json:"git_ref,omitempty"`
}

func NewGeneralMetrics(eventTimestamp *time.Time, appSlug string, originalTrigger string, username string, gitRef string) GeneralMetrics {
	return GeneralMetrics{
		TimeStamp:       time.Now(),
		EventTimestamp:  eventTimestamp,
		AppSlug:         appSlug,
		OriginalTrigger: originalTrigger,
		Username:        username,
		GitRef:          gitRef,
	}
}

// GeneralPullRequestMetrics ...
type GeneralPullRequestMetrics struct {
	PullRequestID  string `json:"pull_request_id,omitempty"` // PR number
	CommitID       string `json:"commit_id,omitempty"`
	ChangedFiles   int    `json:"changed_files_count"`
	Additions      int    `json:"addition_count"`
	Deletions      int    `json:"deletion_count"`
	Commits        int    `json:"commit_count"`
	MergeCommitSHA string `json:"merge_commit_sha,omitempty"`
	Status         string `json:"status,omitempty"`
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
// TODO: remove String() funcs
// String ...
func (m PushMetrics) String() string {
	return stringer(m)
}

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
