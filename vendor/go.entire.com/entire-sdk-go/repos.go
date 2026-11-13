package entire

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strings"
)

// ReposService reads the repository directory and repositories, ranks the
// agents that worked on a repository's code, and reads the CI checks of its
// commits. The directory and repositories are Core's; Experts and
// CommitChecks are the repository's cell's.
type ReposService service

// Repo is a repository: a GitHub repository onboarded to Entire, or one
// hosted by Entire.
type Repo struct {
	// ID is the repository ID that the repository methods take.
	ID string `json:"id"`
	// Name is the repository's name, such as "widget".
	Name string `json:"name"`
	// FullName is the owner and name, such as "acme/widget".
	FullName string `json:"fullName"`
	// Provider is "github" for a GitHub repository or "entire" for one hosted
	// by Entire.
	Provider string `json:"provider"`
	// Visibility is "public" or "private".
	Visibility string `json:"visibility"`
	// State is the repository's provisioning state: "provisioning",
	// "active", "failed", or "deleting". A mirror is active from creation,
	// before its first clone completes. A repository hosted by Entire is
	// "deleting" while its deletion removes its copies in other regions;
	// read it again until Get fails with 404.
	State string `json:"state"`
	// ProjectID is the ID of the project that owns the repository.
	ProjectID string `json:"owningProjectId"`
	// GroupID identifies the repository in every region that holds a copy.
	GroupID string `json:"repoGroupId"`
	// ObjectFormat is the repository's Git hash, "sha1" or "sha256".
	ObjectFormat string `json:"objectFormat"`
	// LastPushedAt is when the repository was last pushed to, or the zero
	// time when that is unknown.
	LastPushedAt Timestamp `json:"lastPushedAt"`
	// DetachedAt is when a detach turned the repository's GitHub mirror
	// into a repository hosted by Entire, or the zero time for a
	// repository that was never detached.
	DetachedAt Timestamp `json:"detachedAt"`
	// Jurisdiction is the repository's home region, such as "us". It is
	// empty when Core does not know it.
	Jurisdiction string `json:"jurisdiction"`
	// ClusterSlug names the cluster that hosts the repository, such as
	// "aws-eu-central-1", or is empty when Core does not know it.
	ClusterSlug string `json:"clusterSlug"`
	// ClusterHost is the public host of that cluster. The repository's
	// entire:// remote is "entire://" + ClusterHost + Path.
	ClusterHost string `json:"clusterHost"`
	// Path is the repository's path on ClusterHost, such as
	// "/et/acme/widget". Only Get sets it.
	Path string `json:"path"`
	// Foreign reports a repository homed in another region than the Core
	// that answered, which knows only its registry entry: State, Path, and
	// ObjectFormat are then empty.
	Foreign bool `json:"foreign"`
	// MirrorSuspended reports a GitHub mirror whose syncing Entire
	// suspended, such as after repeated failures. State stays "active".
	MirrorSuspended bool `json:"mirrorSuspended"`
	// MirrorSuspendedAt is when the mirror was suspended, or the zero time.
	MirrorSuspendedAt Timestamp `json:"mirrorSuspendedAt"`
	// Primaries names the repository's primaries, or is nil for a
	// repository not yet in Core's global registry.
	Primaries *RepoPrimaries `json:"primaries"`
	// Placements are the repository's copies, one per cell. Get sets them
	// when the caller may pull the repository.
	Placements []RepoPlacement `json:"placements"`
	// Capabilities says what the caller may do with the repository.
	Capabilities RepoCapabilities `json:"capabilities"`
}

// RepoPrimaries names a repository's primaries: where its Git data comes
// from, and which copy Entire processes.
type RepoPrimaries struct {
	// GitData identifies the source of the repository's Git data, such as
	// "github:123" for a GitHub repository.
	GitData string `json:"git_data"`
	// Processing is the ID of the copy that Entire processes: the
	// RepoPlacement.ID of one of the placements. It is empty while no copy
	// can be.
	Processing string `json:"processing"`
	// DataPrimary is GitData as a forge and repository ID.
	DataPrimary ForgeRepoIdentity `json:"data_primary"`
	// ProcessingPrimary is Processing as a forge and repository ID, or nil
	// when Processing is empty.
	ProcessingPrimary *ForgeRepoIdentity `json:"processing_primary"`
}

// ForgeRepoIdentity identifies a repository at its forge.
type ForgeRepoIdentity struct {
	// Forge is "github" or "entire".
	Forge string `json:"forge"`
	// RepoID is the repository's ID at the forge.
	RepoID string `json:"repo_id"`
}

// RepoPlacement is one cell's copy of a repository.
type RepoPlacement struct {
	// ID is the copy's repository ID, which the repository methods take.
	ID string `json:"id"`
	// Jurisdiction is the copy's region, such as "us".
	Jurisdiction string `json:"jurisdiction"`
	// Cell is the ID of the cell that holds the copy, such as
	// "aws-us-east-2", or empty when the cluster has no cell yet.
	Cell string `json:"cell"`
	// APIBaseURL is the origin of that cell's Entire API, or empty when Core
	// cannot derive it.
	APIBaseURL string `json:"apiBaseUrl"`
	// ClusterSlug names the cluster that hosts the copy.
	ClusterSlug string `json:"clusterSlug"`
	// Mirror reports a copy that mirrors GitHub, rather than one hosted by
	// Entire.
	Mirror bool `json:"mirror"`
	// Status is the copy's clone state: "processing" while its first clone
	// runs, "ready", "failed", or "suspended".
	Status string `json:"status"`
}

// RepoCapabilities says what the caller may do with a repository.
type RepoCapabilities struct {
	// CanPull reports whether the caller may pull from the repository.
	CanPull bool `json:"canPull"`
	// CanPush reports whether the caller may push to the repository.
	CanPush bool `json:"canPush"`
	// CanManage reports whether the caller may manage the repository.
	CanManage bool `json:"canManage"`
}

// RepoSummary is a repository as the directory lists it, with its activity.
type RepoSummary struct {
	// ID is the repository ID that the repository methods take.
	ID string `json:"id"`
	// Name is the repository's name, such as "widget".
	Name string `json:"name"`
	// FullName is the owner and name, such as "acme/widget".
	FullName string `json:"full_name"`
	// Org is the owner: the GitHub organization or user, or the Entire
	// project.
	Org string `json:"org"`
	// Provider is "github" for a GitHub repository or "entire" for one hosted
	// by Entire.
	Provider string `json:"provider"`
	// Visibility is "public" or "private".
	Visibility string `json:"visibility"`
	// Permission is the caller's access: "read", "write", or "admin".
	Permission string `json:"permission"`
	// Language is the repository's main language, when known.
	Language string `json:"language"`
	// Archived reports an archived repository.
	Archived bool `json:"archived"`
	// Stars counts the repository's GitHub stars.
	Stars int64 `json:"stars"`
	// Forks counts the repository's GitHub forks.
	Forks int64 `json:"forks"`
	// CheckpointCount counts the repository's checkpoints.
	CheckpointCount int64 `json:"checkpointCount"`
	// OpenPRCount counts the repository's open pull requests.
	OpenPRCount int64 `json:"openPrCount"`
	// LastActivityAt is the time of the repository's latest Entire
	// activity, or the zero time when that is unknown.
	LastActivityAt Timestamp `json:"lastActivityAt"`
	// LastPushedAt is when the repository was last pushed to, or the zero
	// time when that is unknown.
	LastPushedAt Timestamp `json:"lastPushedAt"`
	// Primaries names the repository's primaries, or is nil when Core
	// cannot resolve them.
	Primaries *RepoPrimaries `json:"primaries"`
	// Placements are the repository's copies, one per cell. Exactly one
	// has ID as its ID.
	Placements []RepoPlacement `json:"placements"`
}

// RepoListOptions filters ReposService.List. The filters combine.
type RepoListOptions struct {
	// Names selects repositories by path, up to 100 of them: "gh/owner/repo"
	// for a GitHub repository, "et/project/repo" for one hosted by Entire,
	// or "owner/repo" for either. Names match exactly but ignore case, and
	// a name with no repository is left out of the result. With Names,
	// List returns a single page and ignores Sort and the ListOptions.
	Names []string `url:"-"`
	// Query matches a substring of the full name, ignoring case.
	Query string `url:"q"`
	// Org selects the repositories of one owner, matching the whole name
	// but ignoring case.
	Org string `url:"org"`
	// ProjectID selects the repositories of one project.
	ProjectID string `url:"projectId"`
	// Language selects repositories whose main language is exactly this.
	Language string `url:"language"`
	// Visibility selects "public" or "private" repositories.
	Visibility string `url:"visibility"`
	// Permission selects repositories the caller has at least this access
	// to: "read", "write", or "admin".
	Permission string `url:"permission"`
	// Sort orders the listing by one or more keys with directions, such as
	// "org:asc,stars:desc". Keys include name, org, language, stars, forks,
	// checkpoint_count, open_pr_count, last_activity_at, and
	// last_pushed_at. A key without a direction sorts ascending.
	Sort string `url:"sort"`
	// ListOptions selects the page. ListAll pages through every result.
	ListOptions
}

// List returns one page of the repositories that the caller can read, in
// Entire's directory.
func (s *ReposService) List(ctx context.Context, opts *RepoListOptions, reqOpts ...RequestOption) (*Response[[]RepoSummary], error) {
	req := coreRequest(http.MethodGet, nil, "/repos")
	if opts != nil {
		addQuery(req.Query, opts)
		if len(opts.Names) > 0 {
			req.Query.Set("filter", strings.Join(opts.Names, ","))
		}
		opts.ListOptions.add(req.Query, "pageSize", "pageToken")
	}
	type envelope struct {
		Repos         []RepoSummary `json:"repos"`
		NextPageToken string        `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[RepoSummary] {
		return page[RepoSummary]{items: e.Repos, next: e.NextPageToken}
	})
}

// ListAll iterates over every repository that List returns, page by page.
func (s *ReposService) ListAll(ctx context.Context, opts *RepoListOptions, reqOpts ...RequestOption) iter.Seq2[RepoSummary, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]RepoSummary], error) {
		p := o
		p.PageToken = token
		return s.List(ctx, &p, reqOpts...)
	})
}

// Get returns a repository.
func (s *ReposService) Get(ctx context.Context, repoID string, reqOpts ...RequestOption) (*Response[Repo], error) {
	return Call[Repo](ctx, s.client, coreRequest(http.MethodGet, nil, "/repos/%s", repoID), reqOpts...)
}

// ExpertsRequest describes the code that ReposService.Experts ranks agents
// for: either Scopes or Query.
type ExpertsRequest struct {
	// Scopes are up to 25 file paths or directory prefixes, such as
	// "cmd/app/", each at most 500 bytes. A scope matches a file that
	// equals it or begins with it; a leading "./" or "/" is ignored.
	Scopes []string `json:"scopes,omitempty"`
	// Query describes the code in natural language instead, such as
	// "retry handling". It needs code search, and a server without it
	// answers 503 Service Unavailable.
	Query string `json:"query,omitempty"`
	// Branch counts only checkpoints linked to commits on this branch.
	// Empty counts every branch.
	Branch string `json:"branch,omitempty"`
	// Limit caps the agents returned, from 1 to 20; zero selects 8.
	Limit int `json:"limit,omitempty"`
	// EvidenceLimit caps the sessions returned per agent, from 1 to 5; zero
	// selects 3.
	EvidenceLimit int `json:"evidence_limit,omitempty"`
}

// Expert is an agent ranked by how much of the selected code it worked on.
type Expert struct {
	// Agent identifies the agent, such as "claude-code".
	Agent string `json:"agent_id"`
	// AgentLabel names the agent for display.
	AgentLabel string `json:"agent_label"`
	// Models lists the models the agent ran in its sessions on the selected
	// code, sorted.
	Models []string `json:"models"`
	// SessionCount counts the agent's sessions on the selected code.
	SessionCount int64 `json:"session_count"`
	// CheckpointCount counts the checkpoints of those sessions that touched
	// the selected code.
	CheckpointCount int64 `json:"checkpoint_count"`
	// StepCount totals the steps of those sessions, including steps on
	// other code.
	StepCount int64 `json:"step_count"`
	// MatchedFiles lists the files in the selected code that the agent's
	// sessions touched, sorted, up to 50 of them.
	MatchedFiles []string `json:"matched_files"`
	// AgentLines counts the committed lines that the agent wrote, summed
	// over its sessions. It and TotalCommitted are nil when no session
	// has attribution.
	AgentLines *int64 `json:"attribution_agent_lines"`
	// TotalCommitted counts the committed lines of those sessions, of
	// which AgentLines are the agent's, or is nil when no session has
	// attribution.
	TotalCommitted *int64 `json:"attribution_total_committed"`
	// LastActivityAt is the latest activity of the agent's sessions on the
	// selected code.
	LastActivityAt Timestamp `json:"last_activity_at"`
	// Sessions are the agent's sessions that best show its work there, up
	// to the request's EvidenceLimit, best first: by files matched, then
	// checkpoints, then recency.
	Sessions []ExpertSession `json:"sessions"`
}

// ExpertSession is a session that shows an agent's work on selected code.
type ExpertSession struct {
	// SessionID is the session's ID.
	SessionID string `json:"session_id"`
	// DisplayName is the session's title: its custom name, else its
	// generated name, else one derived from its content.
	DisplayName string `json:"display_name"`
	// Agent identifies the session's agent, as Expert.Agent does.
	Agent string `json:"agent"`
	// Model is the model the agent ran, or empty when unknown.
	Model string `json:"model"`
	// CheckpointIDs lists the session's checkpoints that touched the
	// selected code, sorted.
	CheckpointIDs []string `json:"checkpoint_ids"`
	// MatchedFiles lists the files in the selected code that the session
	// touched, sorted, up to 50 of them.
	MatchedFiles []string `json:"matched_files"`
	// LastActivityAt is the session's latest activity.
	LastActivityAt Timestamp `json:"last_activity_at"`
}

// Experts ranks the agents that worked on part of a repository, with
// sessions that show their work.
func (s *ReposService) Experts(ctx context.Context, repoID string, body ExpertsRequest, reqOpts ...RequestOption) (*Response[[]Expert], error) {
	req := repoRequest(http.MethodPost, repoID, body, "/repos/%s/experts", repoID)
	req.Idempotent = true // a read, despite the POST
	type envelope struct {
		Profiles []Expert `json:"profiles"`
	}
	return unwrap(ctx, s.client, req, reqOpts, func(e *envelope) []Expert { return e.Profiles })
}

// CommitChecks are the CI checks reported for one commit.
type CommitChecks struct {
	// CommitSHA is the commit, in lowercase.
	CommitSHA string `json:"commit_sha"`
	// Branch is the branch whose plugin checks were read: the one asked
	// for, or the repository's default branch.
	Branch string `json:"branch"`
	// Availability is "available" when every source answered, which
	// includes reporting no checks; "pending" while GitHub's checks are
	// still loading; or "unavailable" when a source failed or its checks
	// are stale. Checks may be incomplete unless it is "available".
	Availability string `json:"availability"`
	// SourceAvailability gives the Availability of each source it can
	// identify: "github" for a GitHub repository's checks, and each CI
	// plugin's slug for the checks it reported. A source missing from it
	// is not known to be healthy; heed Availability too.
	SourceAvailability map[string]string `json:"source_availability"`
	// Checks are the check runs: GitHub's for a GitHub repository, then
	// those that CI plugins reported.
	Checks []CheckRun `json:"checks"`
}

// CommitChecksOptions selects the branch of ReposService.CommitChecks.
type CommitChecksOptions struct {
	// Branch is the branch to read CI plugins' checks for, such as "main";
	// empty selects the repository's default branch.
	Branch string `url:"branch"`
}

// CommitChecks returns the CI checks reported for a commit, without
// applying any trail's gates. sha is the commit's full SHA. Any caller who
// can read the repository may call it. A malformed SHA or branch fails with
// 400 Bad Request, and a repository the caller cannot read with 404 Not
// Found. When a CI source could not be read, the call still succeeds:
// CommitChecks.Availability is "unavailable", and SourceAvailability names
// the source, so treat the list as incomplete rather than as passing.
func (s *ReposService) CommitChecks(ctx context.Context, repoID, sha string, opts *CommitChecksOptions, reqOpts ...RequestOption) (*Response[CommitChecks], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/commits/%s/checks", repoID, sha)
	addQuery(req.Query, opts)
	return Call[CommitChecks](ctx, s.client, req, reqOpts...)
}

// ResolveRepoID returns the ID of the repository at repoPath, such as
// "gh/acme/widget" for a GitHub repository or "et/acme/widget" for one hosted
// by Entire. A leading slash is ignored, and so is a trailing ".git". The
// path must start with "gh/" or "et/"; Entire no longer resolves the legacy
// "git/project/repo" spelling, so use "et/project/repo" instead. Pass the ID
// to the methods that address a repository.
func (c *Client) ResolveRepoID(ctx context.Context, repoPath string, reqOpts ...RequestOption) (string, error) {
	repoPath = strings.TrimPrefix(repoPath, "/")
	req := newRequest(http.MethodGet, Route{RepoPath: repoPath}, false, nil, "/git/repos/resolve", nil)
	req.Query.Set("repoPath", repoPath)
	if err := checkRepoPath(repoPath); err != nil {
		req.invalid = err
	}
	type resolved struct {
		RepoID string `json:"repoId"`
	}
	resp, err := Call[resolved](ctx, c, req, reqOpts...)
	if err != nil {
		return "", err
	}
	if resp.Data.RepoID == "" {
		return "", fmt.Errorf("entire: repository %q resolved to an empty ID", repoPath)
	}
	// The cell that answered holds the ID, so later calls need no lookup.
	if base, ok := c.routing.cached(Route{RepoPath: repoPath}.key()); ok && c.baseURL == nil {
		c.routing.store(Route{RepoID: resp.Data.RepoID}.key(), base)
	}
	return resp.Data.RepoID, nil
}

// checkRepoPath reports a repository path that Entire cannot resolve: one
// that isn't "gh/owner/repo" or "et/project/repo".
func checkRepoPath(repoPath string) error {
	if repoPath == "" {
		return errors.New("entire: empty repository path")
	}
	parts := strings.Split(repoPath, "/")
	if len(parts) != 3 || (parts[0] != "gh" && parts[0] != "et") || parts[1] == "" || parts[2] == "" {
		return fmt.Errorf("entire: repository path %q is not gh/owner/repo or et/project/repo", repoPath)
	}
	return nil
}
