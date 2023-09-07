package common

import (
	"encoding/json"
	"fmt"
	"time"
)

// GeneralMetrics ...
// TODO: specify json keys
type GeneralMetrics struct {
	TimeStamp       time.Time `json:""`
	EventTimestamp  time.Time `json:""`
	AppSlug         string    `json:""`
	Action          string    `json:""`
	OriginalTrigger string    `json:""`
	Username        string    `json:""`
	GitRef          string    `json:""`
}

// PullRequestMetrics ...
type PullRequestMetrics struct {
	PullRequestID string `json:""` // PR number
	CommitID      string `json:""`
	ChangedFiles  int    `json:""`
	Additions     int    `json:""`
	Deletions     int    `json:""`
	Commits       int    `json:""`
}

// PullRequestOpenedMetrics ...
type PullRequestOpenedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:""`
}

// PullRequestClosedMetrics ...
type PullRequestClosedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:""`
}

// PullRequestUpdatedMetrics ...
type PullRequestUpdatedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:""`
}

// PullRequestCommentMetrics ...
type PullRequestCommentMetrics struct {
	GeneralMetrics
	PullRequestMetrics
}

// PushMetrics ...
type PushMetrics struct {
	GeneralMetrics
	CommitIDAfter         string
	CommitIDBefore        string
	OldestCommitTimestamp *time.Time `json:""`
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
