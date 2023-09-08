package common

import (
	"encoding/json"
	"fmt"
	"time"
)

type Event string

const (
	PullRequestEvent Event = "pull_request"
	PushEvent        Event = "git_push"
)

// PushMetrics ...
type PushMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action string `json:"action,omitempty"`

	GeneralMetrics

	CommitIDAfter         string     `json:"commit_id_before,omitempty"`
	CommitIDBefore        string     `json:"commit_id_after,omitempty"`
	OldestCommitTimestamp *time.Time `json:"oldest_commit_timestamp,omitempty"`
	MasterBranch          string     `json:"master_branch,omitempty"`
}

func NewPushCreatedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics("created", generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, masterBranch)
}

func NewPushDeletedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics("deleted", generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, masterBranch)
}

func NewPushForcedMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics("forced", generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, masterBranch)
}

func NewPushMetrics(generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return newPushMetrics("pushed", generalMetrics, commitIDAfter, commitIDBefore, oldestCommitTimestamp, masterBranch)
}

func newPushMetrics(action string, generalMetrics GeneralMetrics, commitIDAfter string, commitIDBefore string, oldestCommitTimestamp *time.Time, masterBranch string) PushMetrics {
	return PushMetrics{
		Event:                 "push",
		Action:                action,
		GeneralMetrics:        generalMetrics,
		CommitIDAfter:         commitIDAfter,
		CommitIDBefore:        commitIDBefore,
		OldestCommitTimestamp: oldestCommitTimestamp,
		MasterBranch:          masterBranch,
	}
}

// PullRequestOpenedMetrics ...
type PullRequestOpenedMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action string `json:"action,omitempty"`

	GeneralMetrics
	PullRequestMetrics
}

// PullRequestClosedMetrics ...
type PullRequestClosedMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action string `json:"action,omitempty"`

	GeneralMetrics
	PullRequestMetrics
}

// PullRequestUpdatedMetrics ...
type PullRequestUpdatedMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action string `json:"action,omitempty"`

	GeneralMetrics
	PullRequestMetrics
}

// PullRequestCommentMetrics ...
type PullRequestCommentMetrics struct {
	Event  Event  `json:"event,omitempty"`
	Action string `json:"action,omitempty"`

	GeneralMetrics
	PullRequestID string `json:"pull_request_id,omitempty"` // PR number
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

// PullRequestMetrics ...
type PullRequestMetrics struct {
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
func (m PullRequestOpenedMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

// String ...
// TODO: remove String() funcs
func (m PullRequestOpenedMetrics) String() string {
	return stringer(m)
}

// Serialise ...
func (m PullRequestClosedMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

// String ...
func (m PullRequestClosedMetrics) String() string {
	return stringer(m)
}

// Serialise ...
func (m PullRequestUpdatedMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

// String ...
func (m PullRequestUpdatedMetrics) String() string {
	return stringer(m)
}

// Serialise ...
func (m PullRequestCommentMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

// String ...
func (m PullRequestCommentMetrics) String() string {
	return stringer(m)
}

// Serialise ...
func (m PushMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

// String ...
func (m PushMetrics) String() string {
	return stringer(m)
}

func stringer(v interface{}) string {
	c, err := json.MarshalIndent(v, "", "\t")
	if err == nil {
		return string(c)
	}
	return fmt.Sprintf("#%v", v)
}
