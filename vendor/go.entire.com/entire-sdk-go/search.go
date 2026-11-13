package entire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"strconv"
	"strings"
)

// SearchService searches code, and Entire's history of repositories,
// checkpoints, commits, pull requests, and sessions. A search covers the
// repositories that the caller can read in their home region, or, when its
// Repos option names a single repository, that repository wherever it is.
type SearchService service

// CodeSearchOptions narrows Search.Code.
type CodeSearchOptions struct {
	// Repos limits the search to these repository IDs. With exactly one, the
	// search goes to the region that stores that repository.
	Repos []string `url:"repo"`
	// OrgID limits the search to one organization's repositories.
	OrgID string `url:"org_id"`
	// MaxResults caps the matches returned; zero selects 1000.
	MaxResults int `url:"max_results"`
	// CaseSensitive matches case exactly. By default the search ignores
	// case.
	CaseSensitive bool `url:"case_sensitive"`
}

// CodeResults are the results of a code search.
type CodeResults struct {
	// Matches are the matching lines, up to the MaxResults of the
	// CodeSearchOptions.
	Matches []CodeMatch
	// TotalMatches counts every match, which can exceed len(Matches).
	TotalMatches int64
	// TotalFiles counts the files that hold the matches.
	TotalFiles int64
	// ReposSearched counts the repositories searched.
	ReposSearched int64
	// ReposFailed counts the repositories that could not be searched; when
	// it is nonzero, Matches may be incomplete.
	ReposFailed int64
}

// CodeMatch is a line of code that matched a search.
type CodeMatch struct {
	// Repo is the repository's path, such as "gh/acme/widget".
	Repo string `json:"repo"`
	// RepoID is the repository's ID, which the repository methods take.
	RepoID string `json:"repo_id"`
	// Path is the file's path within the repository.
	Path string `json:"path"`
	// Line is the line number of the match, starting at 1.
	Line int64 `json:"line"`
	// Column is the column of the match within Line, starting at 1.
	Column int64 `json:"column"`
	// Text is the matching line.
	Text string `json:"context_line"`
	// Before holds the lines just before Text.
	Before []string `json:"context_before"`
	// After holds the lines just after Text.
	After []string `json:"context_after"`
	// Score is the match's relevance score.
	Score float64 `json:"score"`
}

// Code searches the text of the repositories' default branches.
func (s *SearchService) Code(ctx context.Context, query string, opts *CodeSearchOptions, reqOpts ...RequestOption) (*Response[CodeResults], error) {
	req := homeRequest(http.MethodGet, nil, "/search/api/search")
	req.Query.Set("q", query)
	if query == "" {
		req.invalid = errors.New("entire: Search.Code needs a query")
	}
	if opts != nil {
		addQuery(req.Query, opts)
		req.Route = searchRoute(opts.Repos)
	}
	type response struct {
		Results []CodeMatch `json:"results"`
		Stats   struct {
			TotalMatches  int64 `json:"total_matches"`
			TotalFiles    int64 `json:"total_files"`
			ReposSearched int64 `json:"repos_searched"`
			ReposFailed   int64 `json:"repos_failed"`
		} `json:"stats"`
	}
	return unwrap(ctx, s.client, req, reqOpts, func(r *response) CodeResults {
		return CodeResults{
			Matches:       r.Results,
			TotalMatches:  r.Stats.TotalMatches,
			TotalFiles:    r.Stats.TotalFiles,
			ReposSearched: r.Stats.ReposSearched,
			ReposFailed:   r.Stats.ReposFailed,
		}
	})
}

// SemanticSearchOptions narrows Search.Semantic.
type SemanticSearchOptions struct {
	// Types selects the kinds of results: "repos", "checkpoints",
	// "commits", "prs", or "sessions". None selects them all.
	Types []string `url:"-"`
	// Repos limits the search to these repository IDs, up to 100. The API
	// rejects a full name such as "acme/widget", and answers 404 Not Found
	// when none of the IDs is a repository the caller can read. With
	// exactly one, the search goes to the region that stores it.
	Repos []string `url:"repo"`
	// Branch limits the search to one branch, named in at most 250
	// characters.
	Branch string `url:"branch"`
	// Author limits the search to one author.
	Author string `url:"author"`
	// Date limits the search to the last "week" or "month". Empty means
	// any time.
	Date string `url:"date"`
	// Mode is "hybrid", the default, which matches meaning as well as
	// words, or "keyword".
	Mode string `url:"mode"`
	// Limit caps the results of each kind, from 1 to 500; zero selects 20.
	// In SemanticAll it is the page size.
	Limit int `url:"limit"`
}

// SemanticResults are the results of a semantic search, best first.
type SemanticResults struct {
	// Results are the matches, already in rank order; sorting them by
	// SearchMeta.Score can undo that order.
	Results []SearchResult `json:"results"`
	// Total counts the results that matched, which can exceed Results.
	Total int64 `json:"total"`
	// Truncated reports that Results are a bounded subset rather than
	// every match, because the server capped the repositories it
	// enumerated or stopped early under its memory budget. Retrying does
	// not recover the rest.
	Truncated bool `json:"truncated"`
	// Partial reports that the search could not query every repository it
	// covers, because some failed or timed out, so Results may be
	// incomplete. A repository that has not been indexed yet does not set
	// it.
	Partial bool `json:"partial"`
	// Counts counts the matches of each kind in the whole index, apart from
	// ranking and limits, so they can exceed the results returned.
	Counts SearchKindCounts `json:"counts"`
	// CountsLowerBound reports, for each kind, that its count in Counts is
	// a lower bound rather than exact, because part of the index failed,
	// timed out, or was too large to count.
	CountsLowerBound SearchKindFlags `json:"counts_lower_bound"`
	// TruncatedKinds reports, for each kind, that Limit cut its results
	// short. Only a search of every kind sets it; SemanticAll never does.
	TruncatedKinds SearchKindFlags `json:"truncated_types"`
}

// SearchKindCounts holds a number for each kind of semantic search result.
type SearchKindCounts struct {
	// Repos is the number for repositories.
	Repos int64 `json:"repos"`
	// Checkpoints is the number for checkpoints.
	Checkpoints int64 `json:"checkpoints"`
	// Commits is the number for commits.
	Commits int64 `json:"commits"`
	// PullRequests is the number for pull requests.
	PullRequests int64 `json:"prs"`
	// Sessions is the number for sessions.
	Sessions int64 `json:"sessions"`
}

// SearchKindFlags holds a flag for each kind of semantic search result.
type SearchKindFlags struct {
	// Repos is the flag for repositories.
	Repos bool `json:"repos"`
	// Checkpoints is the flag for checkpoints.
	Checkpoints bool `json:"checkpoints"`
	// Commits is the flag for commits.
	Commits bool `json:"commits"`
	// PullRequests is the flag for pull requests.
	PullRequests bool `json:"prs"`
	// Sessions is the flag for sessions.
	Sessions bool `json:"sessions"`
}

// SearchResult is one result of a semantic search. Type says which of the
// result fields is set.
type SearchResult struct {
	// Type is "repo", "checkpoint", "commit", "pr", or "session". For any
	// other type, only Data and Meta are set.
	Type string
	// Repo is the matching repository when Type is "repo", and nil
	// otherwise.
	Repo *RepoMatch
	// Checkpoint is the matching checkpoint when Type is "checkpoint", and
	// nil otherwise.
	Checkpoint *CheckpointMatch
	// Commit is the matching commit when Type is "commit", and nil
	// otherwise.
	Commit *CommitMatch
	// PullRequest is the matching pull request when Type is "pr", and nil
	// otherwise.
	PullRequest *PullRequestMatch
	// Session is the matching session when Type is "session", and nil
	// otherwise.
	Session *SessionMatch
	// Data is the result as the API sent it, which also holds results of
	// types this SDK doesn't know yet.
	Data json.RawMessage
	// Meta describes how the result matched.
	Meta SearchMeta
}

// UnmarshalJSON decodes the result into the field that its type names.
func (r *SearchResult) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
		Meta SearchMeta      `json:"searchMeta"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*r = SearchResult{Type: wire.Type, Data: wire.Data, Meta: wire.Meta}
	var target any
	switch wire.Type {
	case "repo":
		r.Repo = &RepoMatch{}
		target = r.Repo
	case "checkpoint":
		r.Checkpoint = &CheckpointMatch{}
		target = r.Checkpoint
	case "commit":
		r.Commit = &CommitMatch{}
		target = r.Commit
	case "pr":
		r.PullRequest = &PullRequestMatch{}
		target = r.PullRequest
	case "session":
		r.Session = &SessionMatch{}
		target = r.Session
	default:
		return nil
	}
	if len(bytes.TrimSpace(wire.Data)) == 0 {
		return nil
	}
	return json.Unmarshal(wire.Data, target)
}

// SearchMeta describes how a search result matched.
type SearchMeta struct {
	// MatchType is "keyword", "semantic", "both", or "exact" for an
	// identifier, such as a commit SHA, found exactly.
	MatchType string `json:"matchType"`
	// Score is the result's retrieval score, normalized so that higher is
	// better. It is not necessarily what ordered the results: when the
	// server reranks them, it orders them by another judgment.
	Score float64 `json:"score"`
	// Snippet is the text that matched, or empty when there is none.
	Snippet string `json:"snippet"`
}

// RepoMatch is a repository that matched a search.
type RepoMatch struct {
	// ID is the repository's ID, which the repository methods take.
	ID string `json:"id"`
	// Org is the repository's owner.
	Org string `json:"org"`
	// Name is the repository's name, such as "widget".
	Name string `json:"name"`
	// FullName is the repository's owner and name.
	FullName string `json:"fullName"`
	// Provider is "github" for a GitHub repository or "entire" for one
	// hosted by Entire. Treat an empty Provider, which older servers send,
	// as "github".
	Provider string `json:"provider"`
	// CheckpointCount counts the repository's checkpoints, or is zero when
	// Entire has not observed it.
	CheckpointCount int64 `json:"checkpointCount"`
}

// CheckpointMatch is a checkpoint that matched a search.
type CheckpointMatch struct {
	// ID is the checkpoint's ID.
	ID string `json:"id"`
	// RepoID is the ID of the checkpoint's repository.
	RepoID string `json:"repoId"`
	// Org is the owner of the checkpoint's repository.
	Org string `json:"org"`
	// Repo is the name of the checkpoint's repository.
	Repo string `json:"repo"`
	// Provider is "github" or "entire", like RepoMatch.Provider; treat
	// empty as "github".
	Provider string `json:"provider"`
	// Branch is the checkpoint's branch.
	Branch string `json:"branch"`
	// Prompt is the first prompt of the checkpoint's session.
	Prompt string `json:"prompt"`
	// SessionID is the checkpoint's first session, or empty for a
	// checkpoint indexed before the API reported it.
	SessionID string `json:"sessionId"`
	// CommitSHA is the checkpoint's commit, or empty when there is none.
	CommitSHA string `json:"commitSha"`
	// CommitSubject is the subject line of that commit's message, or
	// empty.
	CommitSubject string `json:"commitSubject"`
	// Author is the checkpoint's author.
	Author string `json:"author"`
	// AuthorLogin is the author's username, or empty when unknown.
	AuthorLogin string `json:"authorUsername"`
	// FilesTouched lists the paths of the files the checkpoint touched.
	FilesTouched []string `json:"filesTouched"`
	// CreatedAt is when the checkpoint was made.
	CreatedAt Timestamp `json:"createdAt"`
}

// CommitMatch is a commit that matched a search.
type CommitMatch struct {
	// SHA is the commit's hash.
	SHA string `json:"commitSha"`
	// RepoID is the ID of the commit's repository.
	RepoID string `json:"repoId"`
	// Org is the owner of the commit's repository.
	Org string `json:"org"`
	// Repo is the name of the commit's repository.
	Repo string `json:"repo"`
	// Provider is "github" or "entire", like RepoMatch.Provider; treat
	// empty as "github".
	Provider string `json:"provider"`
	// Branch is the commit's branch.
	Branch string `json:"branch"`
	// Subject is the subject line of Message.
	Subject string `json:"commitSubject"`
	// Message is the commit's full message.
	Message string `json:"commitMessage"`
	// Author is the commit's author.
	Author string `json:"author"`
	// AuthorLogin is the author's username, or empty when unknown.
	AuthorLogin string `json:"authorUsername"`
	// Additions counts the lines the commit added.
	Additions int64 `json:"additions"`
	// Deletions counts the lines the commit removed.
	Deletions int64 `json:"deletions"`
	// FilesChanged counts the files the commit changed.
	FilesChanged int64 `json:"filesChanged"`
	// HTMLURL is the commit's web page, or empty when there is none.
	HTMLURL string `json:"htmlUrl"`
	// CheckpointID is a checkpoint of the commit that matched, if any.
	CheckpointID string `json:"matchedCheckpointId"`
	// CreatedAt is when the commit was made.
	CreatedAt Timestamp `json:"createdAt"`
}

// PullRequestMatch is a GitHub pull request that matched a search.
type PullRequestMatch struct {
	// Number is the pull request's number in its repository.
	Number int64 `json:"number"`
	// Title is the pull request's title.
	Title string `json:"title"`
	// State is the pull request's state on GitHub.
	State string `json:"state"`
	// Draft reports a draft pull request.
	Draft bool `json:"draft"`
	// HeadBranch is the branch the pull request merges from, or empty when
	// unknown.
	HeadBranch string `json:"headBranch"`
	// Author is the login of the pull request's author, or empty when
	// unknown.
	Author string `json:"userLogin"`
	// Org is the owner of the pull request's repository.
	Org string `json:"org"`
	// Repo is the name of the pull request's repository.
	Repo string `json:"repo"`
	// Provider is "github" or "entire", like RepoMatch.Provider; treat
	// empty as "github".
	Provider string `json:"provider"`
	// HTMLURL is the pull request's web page, or empty when unknown.
	HTMLURL string `json:"htmlUrl"`
	// CreatedAt is when the pull request was created.
	CreatedAt Timestamp `json:"createdAt"`
	// MergedAt is when the pull request was merged, or the zero time if it
	// hasn't been.
	MergedAt Timestamp `json:"mergedAt"`
}

// SessionMatch is an agent session that matched a search.
type SessionMatch struct {
	// SessionID is the session's ID.
	SessionID string `json:"sessionId"`
	// RepoID is the ID of the session's repository.
	RepoID string `json:"repoId"`
	// Org is the owner of the session's repository.
	Org string `json:"org"`
	// Repo is the name of the session's repository.
	Repo string `json:"repo"`
	// Provider is "github" or "entire", like RepoMatch.Provider; treat
	// empty as "github".
	Provider string `json:"provider"`
	// Branch is the session's branch, or empty when unknown.
	Branch string `json:"branch"`
	// DisplayName is the session's title for display.
	DisplayName string `json:"displayName"`
	// Prompt is the session's first prompt, or empty when unknown.
	Prompt string `json:"prompt"`
	// Agent is the agent that ran the session, or empty when unknown.
	Agent string `json:"agent"`
	// Model is the model the agent ran, or empty when unknown.
	Model string `json:"model"`
	// StepCount counts the session's steps.
	StepCount int64 `json:"stepCount"`
	// AuthorLogin is the username of the session's author, or empty when
	// unknown.
	AuthorLogin string `json:"authorUsername"`
	// CommitSHA is the commit of the session's checkpoint that matched, or
	// empty when there is none.
	CommitSHA string `json:"commitSha"`
	// CheckpointID is the session's checkpoint that matched, if any.
	CheckpointID string `json:"matchedCheckpointId"`
	// CreatedAt is when the session was created.
	CreatedAt Timestamp `json:"createdAt"`
}

// Semantic searches Entire's history for query, matching meaning as well as
// words, and returns the best results of each kind.
func (s *SearchService) Semantic(ctx context.Context, query string, opts *SemanticSearchOptions, reqOpts ...RequestOption) (*Response[SemanticResults], error) {
	return s.semantic(ctx, query, opts, "", 0, reqOpts)
}

// semantic sends a semantic search, for one kind and page when kind is set.
func (s *SearchService) semantic(ctx context.Context, query string, opts *SemanticSearchOptions, kind string, pageNumber int, reqOpts []RequestOption) (*Response[SemanticResults], error) {
	req := homeRequest(http.MethodGet, nil, "/semantic-search/search/v1/search")
	req.Query.Set("q", query)
	if query == "" {
		req.invalid = errors.New("entire: Search.Semantic needs a query")
	}
	if opts != nil {
		addQuery(req.Query, opts)
		if len(opts.Types) > 0 {
			req.Query.Set("types", strings.Join(opts.Types, ","))
		}
		req.Route = searchRoute(opts.Repos)
	}
	if kind != "" {
		req.Query.Set("filter_type", kind)
		req.Query.Set("page", strconv.Itoa(pageNumber))
	}
	return Call[SemanticResults](ctx, s.client, req, reqOpts...)
}

// searchRoute sends a search of one repository to the cell that stores it,
// and any other search to the caller's home cell.
func searchRoute(repos []string) Route {
	if len(repos) == 1 {
		return Route{RepoID: repos[0]}
	}
	return Route{}
}

// SemanticAll iterates over every semantic search result of one kind,
// "checkpoints", "commits", or "sessions", page by page; the API pages only
// within a single kind. opts may narrow the search, and its Limit sets the
// page size; its Types are ignored.
func (s *SearchService) SemanticAll(ctx context.Context, query, kind string, opts *SemanticSearchOptions, reqOpts ...RequestOption) iter.Seq2[SearchResult, error] {
	o := snapshot(opts)
	o.Types = nil
	limit := int64(o.Limit)
	if limit <= 0 {
		limit = 20
	}
	return func(yield func(SearchResult, error) bool) {
		seen := int64(0)
		for pageNumber := 1; ; pageNumber++ {
			resp, err := s.semantic(ctx, query, &o, kind, pageNumber, reqOpts)
			if err != nil {
				yield(SearchResult{}, err)
				return
			}
			for _, result := range resp.Data.Results {
				if !yield(result, nil) {
					return
				}
			}
			n := int64(len(resp.Data.Results))
			seen += n
			if n == 0 || n < limit || (resp.Data.Total > 0 && seen >= resp.Data.Total) {
				return
			}
		}
	}
}
