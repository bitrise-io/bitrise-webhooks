package entire

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"time"
)

// SessionsService reads a repository's agent sessions and their
// transcripts.
type SessionsService service

// Session is a coding agent's session: a conversation that produced
// commits, recorded as checkpoints. Sessions.List, Sessions.Get, and
// Me.ListSessions return Session; fields documented as set by one of them are
// empty in the others.
type Session struct {
	// ID identifies the session; pass it to SessionsService methods as
	// sessionID.
	ID string `json:"sessionId"`
	// DisplayName is the session's name: CustomName when someone renamed
	// it, else a name that Entire generated, else the session's prompt or
	// latest commit subject, else "Untitled session". It is at most 200
	// characters.
	DisplayName string `json:"displayName"`
	// CustomName is the name someone gave the session, or empty when no one
	// renamed it.
	CustomName string `json:"customName"`
	// Agent is the coding agent, such as "Claude Code".
	Agent string `json:"agent"`
	// Model is the model that the agent ran, or empty when it is unknown.
	Model string `json:"model"`
	// Kind is the session's purpose, such as "agent_review". It is empty
	// for an ordinary session.
	Kind string `json:"kind"`
	// Prompt is the first line of the session's prompts, or empty when it
	// recorded none.
	Prompt string `json:"prompt"`
	// IsPublic reports whether the session is shared publicly: it has a
	// share link and its repository permits public sharing. Sessions.List
	// and Sessions.Get set it.
	IsPublic bool `json:"isPublic"`
	// FirstCommitAuthor is the GitHub login of the author of the session's
	// earliest commit, or empty when it is unknown.
	FirstCommitAuthor string `json:"firstCommitAuthorUsername"`
	// StartedAt is the date of the session's earliest commit, or of its
	// earliest checkpoint when no commit links it.
	StartedAt Timestamp `json:"startedAt"`
	// LastActivityAt is the date of the session's latest commit, or of its
	// latest checkpoint when no commit links it. Sessions are listed by it,
	// most recent first.
	LastActivityAt Timestamp `json:"lastActivityAt"`
	// CheckpointCount counts the checkpoints that include the session.
	CheckpointCount int64 `json:"checkpointCount"`
	// SubagentCount counts the subagents that the session launched.
	SubagentCount int64 `json:"subagentCount"`
	// StepCount totals the session's steps across its checkpoints, as
	// CheckpointSession.Steps counts them.
	StepCount int64 `json:"stepCount"`
	// InputTokens counts the tokens that the session sent to its model, or
	// is zero when the session reported no usage.
	InputTokens int64 `json:"inputTokens"`
	// OutputTokens counts the tokens that the session's model generated.
	OutputTokens int64 `json:"outputTokens"`
	// CacheCreationTokens counts the tokens that the session wrote to its
	// model's prompt cache.
	CacheCreationTokens int64 `json:"cacheCreationTokens"`
	// CacheReadTokens counts the tokens that the session read from its
	// model's prompt cache.
	CacheReadTokens int64 `json:"cacheReadTokens"`
	// AgentLines counts the committed lines that the agent wrote, summed
	// across the session's checkpoints. It and the other attribution counts
	// are nil when attribution is unknown; see Attribution.
	AgentLines *int64 `json:"attributionAgentLines"`
	// TotalCommitted counts the lines that the session's commits add, net,
	// or is nil when attribution is unknown.
	TotalCommitted *int64 `json:"attributionTotalCommitted"`
	// HumanAdded counts the lines that people added to the session's
	// commits, not counting lines they modified, or is nil when attribution
	// is unknown.
	HumanAdded *int64 `json:"attributionHumanAdded"`
	// HumanModified estimates the lines that people changed in the
	// session's commits, or is nil when attribution is unknown.
	HumanModified *int64 `json:"attributionHumanModified"`
	// HumanRemoved counts the lines that people removed from the session's
	// commits, not counting lines they modified, or is nil when attribution
	// is unknown.
	HumanRemoved *int64 `json:"attributionHumanRemoved"`
	// AttributionCalculatedAt is when the attribution counts were last
	// calculated, or zero when attribution is unknown.
	AttributionCalculatedAt Timestamp `json:"attributionCalculatedAt"`

	// RepoFullName names the session's repository, as "owner/repo", or
	// holds the repository's ID when the name can't be resolved.
	// Me.ListSessions sets it.
	RepoFullName string `json:"repo_full_name"`
	// Provider is the forge of the session's repository: "github" or
	// "entire". Me.ListSessions sets it.
	Provider string `json:"provider"`
	// IsPrivate reports whether the session's repository is private.
	// Me.ListSessions sets it.
	IsPrivate bool `json:"is_private"`
}

// SessionListOptions filters SessionsService.List. By default it lists the
// sessions of the default branch.
type SessionListOptions struct {
	// Branch lists the sessions of this branch instead.
	Branch string `url:"branch"`
	// Head lists the sessions of commits that Head has and Base doesn't,
	// such as a feature branch's. It is a branch or commit SHA. Set Head
	// and Base together, and not Branch or AllBranches.
	Head string `url:"head"`
	// Base is the branch or commit SHA whose commits Head's listing leaves
	// out.
	Base string `url:"base"`
	// AllBranches lists every session ever linked to the repository,
	// including those of deleted or rewritten branches. Don't combine it
	// with Branch, Head, or Base.
	AllBranches bool `url:"-"`
	// Agents selects the sessions of any of these agents.
	Agents []string `url:"agent"`
	// Models selects the sessions of any of these models.
	Models []string `url:"model"`
	// Search matches a substring of the name, prompt, or agent, ignoring
	// case.
	Search string `url:"search"`
	// Since leaves out sessions last active before it. The zero time leaves
	// the start unbounded.
	Since time.Time `url:"since"`
	// Until leaves out sessions last active at or after it. The zero time
	// leaves the end unbounded.
	Until time.Time `url:"until"`
	ListOptions
}

// List returns one page of a repository's sessions, most recently active
// first.
func (s *SessionsService) List(ctx context.Context, repoID string, opts *SessionListOptions, reqOpts ...RequestOption) (*Response[[]Session], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/sessions", repoID)
	if opts != nil {
		addQuery(req.Query, opts)
		if opts.AllBranches {
			req.Query.Set("scope", "all")
		}
		opts.ListOptions.add(req.Query, "limit", "cursor")
	}
	type envelope struct {
		Sessions   []Session `json:"sessions"`
		NextCursor string    `json:"next_cursor"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[Session] {
		return page[Session]{items: e.Sessions, next: e.NextCursor}
	})
}

// ListAll iterates over every session that List returns, page by page.
func (s *SessionsService) ListAll(ctx context.Context, repoID string, opts *SessionListOptions, reqOpts ...RequestOption) iter.Seq2[Session, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Session], error) {
		p := o
		p.PageToken = token
		return s.List(ctx, repoID, &p, reqOpts...)
	})
}

// Get returns a session.
func (s *SessionsService) Get(ctx context.Context, repoID, sessionID string, reqOpts ...RequestOption) (*Response[Session], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/sessions/%s", repoID, sessionID)
	return Call[Session](ctx, s.client, req, reqOpts...)
}

// Transcript returns a session's whole transcript, merged across its
// checkpoints. It fails with ErrTranscriptUnavailable when Entire cannot read
// it.
func (s *SessionsService) Transcript(ctx context.Context, repoID, sessionID string, reqOpts ...RequestOption) (*Response[Transcript], error) {
	type envelope struct {
		Status   string       `json:"status"`
		Sessions []Transcript `json:"sessions"`
	}
	failed := false
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/sessions/%s/transcript", repoID, sessionID)
	resp, err := unwrap(ctx, s.client, req, reqOpts, func(e *envelope) Transcript {
		failed = e.Status == "failed"
		if len(e.Sessions) == 0 {
			return Transcript{SessionID: sessionID}
		}
		return e.Sessions[0]
	})
	if err == nil && failed {
		err = fmt.Errorf("%w: session %s", ErrTranscriptUnavailable, sessionID)
	}
	return resp, err
}
