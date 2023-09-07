package common

import (
	"encoding/json"
	"fmt"
	"time"
)

// GeneralMetrics ...
type GeneralMetrics struct {
	TimeStamp       time.Time  `json:"timestamp,omitempty"`
	EventTimestamp  *time.Time `json:"event_timestamp,omitempty"`
	AppSlug         string     `json:"app_slug,omitempty"`
	Action          string     `json:"action,omitempty"`
	OriginalTrigger string     `json:"original_trigger,omitempty"`
	Username        string     `json:"user_name,omitempty"`
	GitRef          string     `json:"git_ref,omitempty"`
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
}

// PullRequestOpenedMetrics ...
type PullRequestOpenedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:"status,omitempty"`
}

// PullRequestClosedMetrics ...
type PullRequestClosedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:"status,omitempty"`
}

// PullRequestUpdatedMetrics ...
type PullRequestUpdatedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:"status,omitempty"`
}

// PullRequestCommentMetrics ...
type PullRequestCommentMetrics struct {
	GeneralMetrics
	PullRequestID string `json:"pull_request_id,omitempty"` // PR number
}

// PushMetrics ...
type PushMetrics struct {
	GeneralMetrics
	CommitIDAfter         string     `json:"commit_id_before,omitempty"`
	CommitIDBefore        string     `json:"commit_id_after,omitempty"`
	OldestCommitTimestamp *time.Time `json:"oldest_commit_timestamp,omitempty"`
	MasterBranch          string     `json:"master_branch,omitempty"`
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
