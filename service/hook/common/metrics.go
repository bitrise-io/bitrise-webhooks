package common

import (
	"encoding/json"
	"fmt"
	"time"
)

type GeneralMetrics struct {
	Timestamp       time.Time `json:""`
	AppSlug         string    `json:""`
	Action          string    `json:""`
	OriginalTrigger string    `json:""`
	Email           string    `json:""`
	Username        string    `json:""`
	GitRef          string    `json:""`
}

type PullRequestMetrics struct {
	PullRequestID         string     `json:""` // PR number
	CommitID              string     `json:""`
	OldestCommitTimestamp *time.Time `json:""`
	ChangedFiles          int        `json:""`
	Additions             int        `json:""`
	Deletions             int        `json:""`
	Commits               int        `json:""`
}

type PullRequestOpenedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:""`
}

type PullRequestClosedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:""`
}

type PullRequestUpdatedMetrics struct {
	GeneralMetrics
	PullRequestMetrics
	Status string `json:""`
}

type PullRequestCommentMetrics struct {
	GeneralMetrics
	PullRequestMetrics
}

type PushMetrics struct {
	GeneralMetrics
	CommitIDAfter         string
	CommitIDBefore        string
	OldestCommitTimestamp *time.Time `json:""`
}

func (m PullRequestOpenedMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

func (m PullRequestOpenedMetrics) String() string {
	return stringer(m)
}

func (m PullRequestClosedMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

func (m PullRequestClosedMetrics) String() string {
	return stringer(m)
}

func (m PullRequestUpdatedMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

func (m PullRequestUpdatedMetrics) String() string {
	return stringer(m)
}

func (m PullRequestCommentMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

func (m PullRequestCommentMetrics) String() string {
	return stringer(m)
}

func (m PushMetrics) Serialise() ([]byte, error) {
	return json.Marshal(m)
}

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
