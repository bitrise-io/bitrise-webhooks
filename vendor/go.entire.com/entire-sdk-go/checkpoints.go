package entire

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// CheckpointsService reads a repository's checkpoints: the agent sessions
// behind its commits, and their transcripts. To go from a commit to the
// sessions that produced it, call ListForCommit with the commit's SHA, then
// read each Checkpoint's Sessions, or pass a session's ID to
// SessionsService.
type CheckpointsService service

// Checkpoint links a commit to the agent sessions that produced it.
type Checkpoint struct {
	// ID identifies the checkpoint across the repository; pass it to
	// CheckpointsService methods as checkpointID.
	ID string `json:"checkpointId"`
	// CreatedAt is when the checkpoint was created.
	CreatedAt Timestamp `json:"createdAt"`
	// CommitSHA is the commit the checkpoint anchors to. When several
	// commits link the checkpoint, it is the latest of them.
	CommitSHA string `json:"commitSha"`
	// CommitSubject is the first line of CommitMessage.
	CommitSubject string `json:"commitSubject"`
	// CommitMessage is the commit's full message.
	CommitMessage string `json:"commitMessage"`
	// CommitAuthor is the git author name of the commit.
	CommitAuthor string `json:"commitAuthor"`
	// CommitAuthorLogin is the commit author's GitHub login, or empty when
	// it is unknown.
	CommitAuthorLogin string `json:"commitAuthorUsername"`
	// CommitDate is the commit's date.
	CommitDate Timestamp `json:"commitDate"`
	// Additions counts the lines that the commit adds. It and Deletions
	// and FilesChanged are nil when Entire could not diff the commit.
	Additions *int64 `json:"additions"`
	// Deletions counts the lines that the commit deletes, or is nil.
	Deletions *int64 `json:"deletions"`
	// FilesChanged counts the files that the commit changes, or is nil.
	FilesChanged *int64 `json:"filesChanged"`
	// FilesTouched lists the paths of the files that the checkpoint's
	// sessions touched, as the Entire CLI recorded them.
	FilesTouched []string `json:"filesTouched"`
	// Agents are the distinct coding agents of the checkpoint's sessions,
	// such as "Claude Code", in the order they first appear in Sessions.
	Agents []string `json:"agents"`
	// Sessions are the agent sessions behind the commit.
	Sessions []CheckpointSession `json:"sessions"`
	// SessionCount counts the entries of Sessions.
	SessionCount int64 `json:"sessionCount"`
	// TotalSteps totals the Steps of the entries of Sessions.
	TotalSteps int64 `json:"totalSteps"`
	// InputTokens counts the tokens that the sessions sent to their models.
	InputTokens int64 `json:"inputTokens"`
	// OutputTokens counts the tokens that the sessions' models generated.
	OutputTokens int64 `json:"outputTokens"`
	// CacheCreationTokens counts the tokens that the sessions wrote to their
	// models' prompt caches.
	CacheCreationTokens int64 `json:"cacheCreationTokens"`
	// CacheReadTokens counts the tokens that the sessions read from their
	// models' prompt caches.
	CacheReadTokens int64 `json:"cacheReadTokens"`
	// APICallCount counts the sessions' calls to their models' APIs.
	APICallCount int64 `json:"apiCallCount"`
	// Attribution measures how much of the commit the agents wrote, or is
	// nil when it is unknown.
	Attribution *Attribution `json:"attribution"`
	// Branches lists the branches that contain the commit. Only Get sets
	// it, and the Entire API reports only the default branch: Branches
	// holds its name when the default branch contains the commit, and is
	// empty otherwise.
	Branches []string `json:"-"`
}

// CheckpointSession is an agent session behind a checkpoint.
type CheckpointSession struct {
	// SessionID identifies the session; pass it to SessionsService methods
	// as sessionID.
	SessionID string `json:"sessionId"`
	// DisplayName is the session's name, as Session.DisplayName describes
	// it. It is empty when the name can't be resolved.
	DisplayName string `json:"displayName"`
	// Agent is the coding agent, such as "Claude Code", or "Unknown" when
	// the session recorded none.
	Agent string `json:"agent"`
	// Model is the model that the agent ran, such as
	// "claude-sonnet-4-20250514", or empty when it is unknown.
	Model string `json:"model"`
	// Kind is the session's purpose, such as "agent_review". It is empty
	// for an ordinary session.
	Kind string `json:"kind"`
	// Prompt is the first line of the session's prompts, or empty when it
	// recorded none.
	Prompt string `json:"prompt"`
	// Steps counts the steps that the session recorded in the checkpoint.
	// It is at least 1.
	Steps int64 `json:"steps"`
	// CreatedAt is when the Entire CLI recorded the session's part of the
	// checkpoint.
	CreatedAt Timestamp `json:"createdAt"`
	// Attribution measures how much of the commit this session wrote, or
	// is nil when it is unknown.
	Attribution *Attribution `json:"attribution"`
	// TokenUsage counts the tokens the session used in the checkpoint, or
	// is nil when the session reported none.
	TokenUsage *CheckpointTokenUsage `json:"tokenUsage"`
}

// CheckpointTokenUsage counts the tokens that a session used in a
// checkpoint.
type CheckpointTokenUsage struct {
	// InputTokens counts the tokens sent to the model.
	InputTokens int64 `json:"inputTokens"`
	// OutputTokens counts the tokens the model generated.
	OutputTokens int64 `json:"outputTokens"`
	// CacheCreationTokens counts the tokens written to the model's prompt
	// cache.
	CacheCreationTokens int64 `json:"cacheCreationTokens"`
	// CacheReadTokens counts the tokens read from the model's prompt cache.
	CacheReadTokens int64 `json:"cacheReadTokens"`
	// APICallCount counts the calls the session made to the model.
	APICallCount int64 `json:"apiCallCount"`
}

// CheckpointListOptions filters CheckpointsService.List.
type CheckpointListOptions struct {
	// Branch lists the checkpoints of this branch's commits; empty selects
	// the default branch.
	Branch string `url:"branch"`
	// ExcludeBranch leaves out commits that this branch also contains, such
	// as a feature branch's base.
	ExcludeBranch string `url:"excludeBranch"`
	// Since leaves out checkpoints whose commit is dated before it. The
	// zero time leaves the start unbounded.
	Since time.Time `url:"since"`
	// Until leaves out checkpoints whose commit is dated at or after it.
	// The zero time leaves the end unbounded.
	Until time.Time `url:"until"`
	// Limit caps the checkpoints returned, from 1 to 1000; zero selects
	// 100, and larger values select 1000.
	Limit int `url:"limit"`
}

// checkpointList is the envelope of the checkpoint listings.
type checkpointList struct {
	Checkpoints []Checkpoint `json:"checkpoints"`
}

// List returns the newest checkpoints of a branch's commits.
func (s *CheckpointsService) List(ctx context.Context, repoID string, opts *CheckpointListOptions, reqOpts ...RequestOption) (*Response[[]Checkpoint], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/checkpoints", repoID)
	addQuery(req.Query, opts)
	return unwrap(ctx, s.client, req, reqOpts, func(e *checkpointList) []Checkpoint { return e.Checkpoints })
}

// ListForCommit returns the checkpoints of a commit, given its full SHA:
// the agent sessions behind it, in each Checkpoint's Sessions. A commit that
// no agent session produced has none.
func (s *CheckpointsService) ListForCommit(ctx context.Context, repoID, sha string, reqOpts ...RequestOption) (*Response[[]Checkpoint], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/commits/%s/checkpoints", repoID, sha)
	return unwrap(ctx, s.client, req, reqOpts, func(e *checkpointList) []Checkpoint { return e.Checkpoints })
}

// Get returns a checkpoint, with the branches that contain its commit.
func (s *CheckpointsService) Get(ctx context.Context, repoID, checkpointID string, reqOpts ...RequestOption) (*Response[Checkpoint], error) {
	type resolved struct {
		Checkpoint Checkpoint `json:"checkpoint"`
		Branches   []string   `json:"branches"`
	}
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/checkpoints/%s", repoID, checkpointID)
	return unwrap(ctx, s.client, req, reqOpts, func(r *resolved) Checkpoint {
		r.Checkpoint.Branches = r.Branches
		return r.Checkpoint
	})
}

// Transcript returns the transcripts of a checkpoint's sessions, one per
// session. It fails with ErrTranscriptUnavailable when Entire cannot read
// any of them.
func (s *CheckpointsService) Transcript(ctx context.Context, repoID, checkpointID string, reqOpts ...RequestOption) (*Response[[]Transcript], error) {
	type envelope struct {
		Status   string       `json:"status"`
		Sessions []Transcript `json:"sessions"`
	}
	failed := false
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/checkpoints/%s/transcript", repoID, checkpointID)
	resp, err := unwrap(ctx, s.client, req, reqOpts, func(e *envelope) []Transcript {
		failed = e.Status == "failed"
		return e.Sessions
	})
	if err == nil && failed {
		err = fmt.Errorf("%w: checkpoint %s", ErrTranscriptUnavailable, checkpointID)
	}
	return resp, err
}

// RawTranscript streams the transcript of one of a checkpoint's sessions as
// the agent recorded it, such as Claude Code's JSONL. session is the
// session's index in the checkpoint's Sessions, starting at 0. The caller
// must close the returned reader.
func (s *CheckpointsService) RawTranscript(ctx context.Context, repoID, checkpointID string, session int, reqOpts ...RequestOption) (io.ReadCloser, error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/checkpoints/%s/transcript/raw", repoID, checkpointID)
	req.Query.Set("session", strconv.Itoa(session))
	resp, err := s.client.Do(ctx, req, reqOpts...)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
