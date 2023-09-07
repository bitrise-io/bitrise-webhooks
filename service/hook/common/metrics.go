package common

import (
	"encoding/json"
	"fmt"
	"time"
)

// GeneralMetrics ...
type GeneralMetrics struct {
	TimeStamp       time.Time  `json:"timestamp"`
	EventTimestamp  *time.Time `json:"event_timestamp"`
	AppSlug         string     `json:"app_slug"`
	Action          string     `json:"action"`
	OriginalTrigger string     `json:"original_trigger"`
	Username        string     `json:"user_name"`
	GitRef          string     `json:"git_ref"`
}

// PullRequestMetrics ...
type PullRequestMetrics struct {
	PullRequestID  string `json:"pull_request_id"` // PR number
	CommitID       string `json:"commit_id"`
	ChangedFiles   int    `json:"changed_files_count"`
	Additions      int    `json:"addition_count"`
	Deletions      int    `json:"deletion_count"`
	Commits        int    `json:"commit_count"`
	MergeCommitSHA string `json:"merge_commit_sha"`
}

// PullRequestOpenedMetrics ...
type PullRequestOpenedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:"status"`
}

// PullRequestClosedMetrics ...
type PullRequestClosedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:"status"`
}

// PullRequestUpdatedMetrics ...
type PullRequestUpdatedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:"status"`
}

// PullRequestCommentMetrics ...
type PullRequestCommentMetrics struct {
	GeneralMetrics
	PullRequestID string `json:"pull_request_id"` // PR number
}

// PushMetrics ...
type PushMetrics struct {
	GeneralMetrics
	CommitIDAfter         string     `json:"commit_id_before"`
	CommitIDBefore        string     `json:"commit_id_after"`
	OldestCommitTimestamp *time.Time `json:"oldest_commit_timestamp"`
	MasterBranch          string     `json:"master_branch"`
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
