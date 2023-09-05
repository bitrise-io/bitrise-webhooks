package common

import (
	"encoding/json"
	"fmt"
	"time"
)

type PullRequestMetrics struct {
	Timestamp             time.Time  `json:""`
	AppSlug               string     `json:""`
	Action                string     `json:""`
	PullRequestID         string     `json:""` // PR number
	Email                 string     `json:""`
	Username              string     `json:""`
	GitRef                string     `json:""`
	CommitID              string     `json:""`
	OriginalTrigger       string     `json:""`
	OldestCommitTimestamp *time.Time `json:""`
	ChangedFiles          int
	Additions             int
	Deletions             int
	Commits               int
}

type PullRequestOpenedMetrics struct {
	PullRequestMetrics
	Status string `json:""`
}

func (m PullRequestOpenedMetrics) String() string {
	return stringer(m)
}

type PullRequestClosedMetrics struct {
	PullRequestMetrics
	Status string `json:""`
}

func (m PullRequestClosedMetrics) String() string {
	return stringer(m)
}

type PullRequestUpdatedMetrics struct {
	PullRequestOpenedMetrics
}

func (m PullRequestUpdatedMetrics) String() string {
	return stringer(m)
}

type PullRequestCommentMetrics struct {
	PullRequestMetrics
}

func (m PullRequestCommentMetrics) String() string {
	return stringer(m)
}

type PushMetrics struct {
	Timestamp             time.Time `json:""`
	AppSlug               string    `json:""`
	Action                string    `json:""`
	Email                 string    `json:""`
	Username              string    `json:""`
	GitRef                string    `json:""`
	CommitIDAfter         string
	CommitIDBefore        string
	OriginalTrigger       string     `json:""`
	OldestCommitTimestamp *time.Time `json:""`
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
