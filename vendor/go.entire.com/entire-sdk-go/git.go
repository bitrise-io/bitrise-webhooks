package entire

import (
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"
	"time"
)

// GitService reads a repository's Git data: branches, tags, and other refs,
// commits, trees, files, and diffs. It also creates branches and watches refs
// change. Methods that take a ref accept a branch, tag, or commit SHA,
// optionally with a revision suffix such as "main~2" or "<sha>^".
type GitService service

// Ref is a branch or tag and the commit it points to.
type Ref struct {
	// Name is the branch or tag name, without its "refs/heads/" or
	// "refs/tags/" prefix, such as "main" or "v1.2.0". CreateBranch
	// returns the fully qualified name instead.
	Name string `json:"name"`
	// CommitSHA is the full hex SHA of the object the ref points to: a
	// commit for a branch or a lightweight tag, and the tag object for an
	// annotated tag.
	CommitSHA string `json:"commitSHA"`
}

// RefEntry is a ref as ListRefs reports it.
type RefEntry struct {
	// Name is the fully qualified name, such as "HEAD", "refs/heads/main",
	// or "refs/tags/v1".
	Name string `json:"name"`
	// OID is the object the ref points to: a commit for a branch, the tag
	// object for an annotated tag. It is empty for an unborn HEAD.
	OID string `json:"oid"`
	// Peeled is the object an annotated tag points to, usually a commit,
	// with RefListOptions.Peel. It is empty for other refs.
	Peeled string `json:"peeled"`
	// SymrefTarget is the target of a symbolic ref, such as
	// "refs/heads/main" for HEAD, with RefListOptions.Symrefs.
	SymrefTarget string `json:"symrefTarget"`
	// Unborn reports a HEAD whose branch has no commits yet, with
	// RefListOptions.Unborn.
	Unborn bool `json:"unborn"`
}

// Commit is a Git commit.
type Commit struct {
	// SHA is the commit's full hex SHA: 40 characters for SHA-1, or 64 for
	// SHA-256.
	SHA string `json:"sha"`
	// Message is the full commit message, including any trailers.
	Message string `json:"message"`
	// Author is who wrote the change, and when.
	Author Signature `json:"author"`
	// Committer is who created the commit, and when. ListCommits filters
	// on the committer's date.
	Committer Signature `json:"committer"`
	// Parents are the SHAs of the commit's parents, first parent first. A
	// root commit has none.
	Parents []string `json:"parents"`
	// Stats summarizes the commit's diff against its first parent. GetCommit
	// sets it, as does ListCommits with CommitListOptions.IncludeStats. It
	// is nil when computing it took too long.
	Stats *CommitStats `json:"stats"`
	// Trailers are the message's Git trailers, in order. GetCommit sets
	// them, as does ListCommits with CommitListOptions.ParseTrailers.
	Trailers []Trailer `json:"trailers"`
	// Signature is the commit's GPG or SSH signature, if it is signed.
	Signature string `json:"signature"`
}

// Signature is a commit's author or committer.
type Signature struct {
	// Name is the person's name as recorded in the commit.
	Name string `json:"name"`
	// Email is the person's email address as recorded in the commit.
	Email string `json:"email"`
	// Date is when the person authored or committed the change.
	Date Timestamp `json:"date"`
}

// CommitStats summarizes a commit's diff against its first parent.
type CommitStats struct {
	// Additions is the number of lines added.
	Additions int64 `json:"additions"`
	// Deletions is the number of lines removed.
	Deletions int64 `json:"deletions"`
	// FilesChanged is the number of files the commit changed.
	FilesChanged int64 `json:"filesChanged"`
	// Truncated reports that the diff exceeded the API's file or line limit,
	// so the counts are lower bounds.
	Truncated bool `json:"truncated"`
}

// Trailer is a Git trailer, such as "Co-Authored-By: …".
type Trailer struct {
	// Key is the trailer's token, such as "Co-Authored-By".
	Key string `json:"key"`
	// Value is the text after the trailer's separator.
	Value string `json:"value"`
}

// Comparison is the set of files that differ between two refs.
type Comparison struct {
	// Files are the files that differ.
	Files []ChangedFile `json:"files"`
	// Truncated reports that the comparison exceeded the API's file or line
	// limit, so Files is incomplete. The line limit applies only when
	// CompareOptions.IncludeStats is in effect.
	Truncated bool `json:"truncated"`
}

// ChangedFile is a file that a comparison found changed.
type ChangedFile struct {
	// Path is the file's path from the repository root; for a rename, its
	// new path.
	Path string `json:"path"`
	// PreviousPath is the file's path before a rename, and empty unless
	// Status is "renamed".
	PreviousPath string `json:"previousPath"`
	// Status is "added", "modified", "removed", or "renamed".
	Status string `json:"status"`
	// Additions is the number of lines added. It is zero for a Binary or
	// TooLarge file, and when CompareOptions.IncludeStats is false.
	Additions int64 `json:"additions"`
	// Deletions is the number of lines removed. It is zero for a Binary or
	// TooLarge file, and when CompareOptions.IncludeStats is false.
	Deletions int64 `json:"deletions"`
	// Binary reports that the file is binary on either side. It is false
	// when CompareOptions.IncludeStats is false.
	Binary bool `json:"binary"`
	// TooLarge reports that one side of the file exceeded the API's diff
	// size limit and was not read; the file changed, by an unknown extent.
	TooLarge bool `json:"tooLarge"`
}

// Tree is a directory listing at a ref.
type Tree struct {
	// SHA is the SHA of the tree object listed: the root tree, or the
	// directory at TreeOptions.Path.
	SHA string `json:"sha"`
	// Entries are the files, directories, and submodules in the tree.
	Entries []TreeEntry `json:"entries"`
	// Truncated reports that the tree has more than the API's limit of
	// 100,000 entries, so Entries is incomplete.
	Truncated bool `json:"truncated"`
}

// TreeEntry is a file, directory, or submodule in a tree.
type TreeEntry struct {
	// Path is the entry's path relative to the directory listed, such as
	// "main.go", or "cmd/app/main.go" in a recursive listing.
	Path string `json:"path"`
	// Type is "blob" for a file, "tree" for a directory, or "commit" for a
	// submodule.
	Type string `json:"type"`
	// Mode is the Git file mode, as six octal digits, such as "100644".
	Mode string `json:"mode"`
	// SHA is the SHA of the entry's object: a blob, a tree, or, for a
	// submodule, a commit.
	SHA string `json:"sha"`
	// Size is a file's size in bytes. It is zero for a directory or
	// submodule.
	Size int64 `json:"size"`
}

// RefSearchOptions filters ListBranches and ListTags by name.
type RefSearchOptions struct {
	// Search matches names containing this substring, ignoring case. A
	// leading "^" or trailing "$" anchors it to the start or end of the
	// name. It cannot be combined with Regex.
	Search string `url:"search"`
	// Regex matches names against this regular expression, in Go's RE2
	// syntax. It cannot be combined with Search.
	Regex string `url:"regex"`
	ListOptions
}

// RefListOptions filters ListRefs and adds detail to its entries.
type RefListOptions struct {
	// Prefixes selects refs whose names start with one of these, such as
	// "refs/heads/", "refs/tags/", or "HEAD". Empty selects every ref.
	Prefixes []string `url:"prefix"`
	// Symrefs sets the target of symbolic refs, such as HEAD.
	Symrefs bool `url:"symrefs"`
	// Peel sets the commit that each annotated tag points to.
	Peel bool `url:"peel"`
	// Unborn lists a HEAD whose branch has no commits yet.
	Unborn bool `url:"unborn"`
	ListOptions
}

// CommitListOptions filters ListCommits.
type CommitListOptions struct {
	// NotRef excludes commits reachable from this ref, such as a base
	// branch, to list only those unique to the listed ref.
	NotRef string `url:"notRef"`
	// Since selects commits committed at or after this time. Zero sets no
	// bound.
	Since time.Time `url:"since"`
	// Until selects commits committed before this time. Zero sets no bound.
	Until time.Time `url:"until"`
	// Path selects commits that changed this file or anything below this
	// directory, compared with their first parent.
	Path string `url:"path"`
	// Author selects commits whose author name or email contains this
	// substring, ignoring case.
	Author string `url:"author"`
	// FirstParent follows only the first parent of merge commits.
	FirstParent bool `url:"firstParent"`
	// IncludeStats sets each commit's Stats.
	IncludeStats bool `url:"includeStats"`
	// ParseTrailers sets each commit's Trailers.
	ParseTrailers bool `url:"parseTrailers"`
	ListOptions
}

// CompareOptions narrows Compare.
type CompareOptions struct {
	// Path compares only this file or directory.
	Path string `url:"path"`
	// IncludeStats counts each file's additions and deletions and detects
	// binary files. It defaults to true; set it to Ptr(false) for paths and
	// statuses only.
	IncludeStats *bool `url:"includeStats"`
}

// TreeOptions selects what GetTree lists.
type TreeOptions struct {
	// Path lists this directory, such as "cmd/app", instead of the root.
	Path string `url:"path"`
	// Recursive lists every file and directory below the listed directory,
	// not only its immediate entries.
	Recursive bool `url:"recursive"`
}

// DiffOptions narrows Diff.
type DiffOptions struct {
	// Path diffs only this file or directory.
	Path string `url:"path"`
}

// ListBranches returns one page of the repository's branches, ordered by
// name.
func (s *GitService) ListBranches(ctx context.Context, repoID string, opts *RefSearchOptions, reqOpts ...RequestOption) (*Response[[]Ref], error) {
	return s.listRefs(ctx, repoID, "branches", opts, reqOpts)
}

// ListBranchesAll iterates over every branch that ListBranches returns, page
// by page.
func (s *GitService) ListBranchesAll(ctx context.Context, repoID string, opts *RefSearchOptions, reqOpts ...RequestOption) iter.Seq2[Ref, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Ref], error) {
		p := o
		p.PageToken = token
		return s.ListBranches(ctx, repoID, &p, reqOpts...)
	})
}

// ListTags returns one page of the repository's tags, ordered by name.
func (s *GitService) ListTags(ctx context.Context, repoID string, opts *RefSearchOptions, reqOpts ...RequestOption) (*Response[[]Ref], error) {
	return s.listRefs(ctx, repoID, "tags", opts, reqOpts)
}

// ListTagsAll iterates over every tag that ListTags returns, page by page.
func (s *GitService) ListTagsAll(ctx context.Context, repoID string, opts *RefSearchOptions, reqOpts ...RequestOption) iter.Seq2[Ref, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Ref], error) {
		p := o
		p.PageToken = token
		return s.ListTags(ctx, repoID, &p, reqOpts...)
	})
}

// listRefs lists branches or tags, whose envelopes differ only in the name of
// their list.
func (s *GitService) listRefs(ctx context.Context, repoID, kind string, opts *RefSearchOptions, reqOpts []RequestOption) (*Response[[]Ref], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/git/repos/%s/"+kind, repoID)
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "pageSize", "pageToken")
	}
	type envelope struct {
		Branches      []Ref  `json:"branches"`
		Tags          []Ref  `json:"tags"`
		NextPageToken string `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[Ref] {
		return page[Ref]{items: append(e.Branches, e.Tags...), next: e.NextPageToken}
	})
}

// ListRefs returns one page of the repository's refs: branches, tags, HEAD,
// and any others, ordered by name. With Prefixes "HEAD" and Symrefs set, it
// reports the default branch as HEAD's SymrefTarget.
func (s *GitService) ListRefs(ctx context.Context, repoID string, opts *RefListOptions, reqOpts ...RequestOption) (*Response[[]RefEntry], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/git/repos/%s/refs", repoID)
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "pageSize", "pageToken")
	}
	type envelope struct {
		Refs          []RefEntry `json:"refs"`
		NextPageToken string     `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[RefEntry] {
		return page[RefEntry]{items: e.Refs, next: e.NextPageToken}
	})
}

// ListRefsAll iterates over every ref that ListRefs returns, page by page.
func (s *GitService) ListRefsAll(ctx context.Context, repoID string, opts *RefListOptions, reqOpts ...RequestOption) iter.Seq2[RefEntry, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]RefEntry], error) {
		p := o
		p.PageToken = token
		return s.ListRefs(ctx, repoID, &p, reqOpts...)
	})
}

// BranchCreateRequest is the body of GitService.CreateBranch.
type BranchCreateRequest struct {
	// Name may be bare, such as "feature/x", or fully qualified, such as
	// "refs/heads/feature/x".
	Name string `json:"branch"`
	// CommitSHA is the commit to branch from, which the repository must
	// already have.
	CommitSHA string `json:"commitSHA"`
}

// CreateBranch creates a branch and returns it with its fully qualified name.
func (s *GitService) CreateBranch(ctx context.Context, repoID string, body BranchCreateRequest, reqOpts ...RequestOption) (*Response[Ref], error) {
	type created struct {
		Branch    string `json:"branch"`
		CommitSHA string `json:"commitSHA"`
	}
	req := repoRequest(http.MethodPost, repoID, body, "/git/repos/%s/branches", repoID)
	return unwrap(ctx, s.client, req, reqOpts, func(c *created) Ref {
		return Ref{Name: c.Branch, CommitSHA: c.CommitSHA}
	})
}

// ListCommits returns one page of the commits reachable from ref, newest
// first.
func (s *GitService) ListCommits(ctx context.Context, repoID, ref string, opts *CommitListOptions, reqOpts ...RequestOption) (*Response[[]Commit], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/git/repos/%s/commits", repoID)
	req.Query.Set("ref", ref)
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "pageSize", "pageToken")
	}
	type envelope struct {
		Commits       []Commit `json:"commits"`
		NextPageToken string   `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[Commit] {
		return page[Commit]{items: e.Commits, next: e.NextPageToken}
	})
}

// ListCommitsAll iterates over every commit that ListCommits returns, page by
// page.
func (s *GitService) ListCommitsAll(ctx context.Context, repoID, ref string, opts *CommitListOptions, reqOpts ...RequestOption) iter.Seq2[Commit, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Commit], error) {
		p := o
		p.PageToken = token
		return s.ListCommits(ctx, repoID, ref, &p, reqOpts...)
	})
}

// GetCommit returns the commit with the given full SHA, with its Stats and
// Trailers set.
func (s *GitService) GetCommit(ctx context.Context, repoID, sha string, reqOpts ...RequestOption) (*Response[Commit], error) {
	// The API reads commits in batches only; this is a batch of one.
	body := map[string]any{"shas": []string{sha}, "includeStats": true, "parseTrailers": true}
	req := repoRequest(http.MethodPost, repoID, body, "/git/repos/%s/commits/batch", repoID)
	req.Idempotent = true // a read, despite the POST
	resp, err := Call[map[string]Commit](ctx, s.client, req, reqOpts...)
	if resp == nil {
		return nil, err
	}
	out := &Response[Commit]{StatusCode: resp.StatusCode, Header: resp.Header, HTTPResponse: resp.HTTPResponse}
	if err != nil {
		return out, err
	}
	commit, found := resp.Data[sha]
	for key, c := range resp.Data {
		if !found && strings.EqualFold(key, sha) {
			commit, found = c, true
		}
	}
	if !found {
		return out, &HTTPError{StatusCode: http.StatusNotFound, Message: fmt.Sprintf("commit %s not found", sha), Header: resp.Header}
	}
	out.Data = commit
	return out, nil
}

// Compare returns the files that differ between base and head.
func (s *GitService) Compare(ctx context.Context, repoID, base, head string, opts *CompareOptions, reqOpts ...RequestOption) (*Response[Comparison], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/git/repos/%s/compare", repoID)
	req.Query.Set("base", base)
	req.Query.Set("head", head)
	addQuery(req.Query, opts)
	return Call[Comparison](ctx, s.client, req, reqOpts...)
}

// MergeBase returns the SHA of the best common ancestor of refs a and b, or
// an empty SHA when they share no history.
func (s *GitService) MergeBase(ctx context.Context, repoID, a, b string, reqOpts ...RequestOption) (*Response[string], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/git/repos/%s/merge-base", repoID)
	req.Query.Set("commitA", a)
	req.Query.Set("commitB", b)
	type mergeBase struct {
		SHA string `json:"mergeBaseSHA"`
	}
	return unwrap(ctx, s.client, req, reqOpts, func(m *mergeBase) string { return m.SHA })
}

// GetTree lists a directory at ref: the root, or TreeOptions.Path.
func (s *GitService) GetTree(ctx context.Context, repoID, ref string, opts *TreeOptions, reqOpts ...RequestOption) (*Response[Tree], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/git/repos/%s/tree", repoID)
	req.Query.Set("ref", ref)
	addQuery(req.Query, opts)
	return Call[Tree](ctx, s.client, req, reqOpts...)
}

// OpenFile streams the contents of the file at path, such as
// "cmd/app/main.go", at ref. The caller must close the returned reader.
func (s *GitService) OpenFile(ctx context.Context, repoID, ref, path string, reqOpts ...RequestOption) (io.ReadCloser, error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/git/repos/%s/raw/%s", repoID, filePath(path))
	req.Query.Set("ref", ref)
	resp, err := s.client.Do(ctx, req, reqOpts...)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ReadFile returns the contents of the file at path at ref, buffered in
// memory up to the client's response size limit (WithMaxResponseBytes, 32 MiB
// by default). Stream larger files with OpenFile.
func (s *GitService) ReadFile(ctx context.Context, repoID, ref, path string, reqOpts ...RequestOption) ([]byte, error) {
	body, err := s.OpenFile(ctx, repoID, ref, path, reqOpts...)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := readLimited(body, s.client.maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("entire: read %s: %w", path, err)
	}
	return data, nil
}

// Diff returns the unified diff from base to head, buffered in memory up to
// the client's response size limit.
func (s *GitService) Diff(ctx context.Context, repoID, base, head string, opts *DiffOptions, reqOpts ...RequestOption) ([]byte, error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/git/repos/%s/diff", repoID)
	req.Query.Set("base", base)
	req.Query.Set("head", head)
	addQuery(req.Query, opts)
	resp, err := s.client.Do(ctx, req, reqOpts...)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)
	data, err := readLimited(resp.Body, s.client.maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("entire: read diff: %w", err)
	}
	return data, nil
}

// RefEvent is what WatchRefs yields: the refs as they are when the stream
// connects, or one change to a ref.
type RefEvent struct {
	// Snapshot lists every ref the watch covers. It is set on the first
	// event of each connection, including after a reconnect, since the
	// stream does not replay what it missed; replace the refs you track
	// with it. It is nil on a change.
	Snapshot []RefEntry
	// Change is a change to one ref, when Snapshot is nil.
	Change *RefChange
}

// RefChange is a change to a ref.
type RefChange struct {
	// Ref is the fully qualified ref name, such as "refs/heads/main".
	Ref string `json:"refName"`
	// Op is "create", "update", "force_update", "delete", or
	// "symref_update".
	Op string `json:"refOp"`
	// OldTarget is the object the ref pointed to before the change. It is
	// empty on create.
	OldTarget string `json:"oldTarget"`
	// NewTarget is the object the ref points to after the change. It is
	// empty on delete.
	NewTarget string `json:"newTarget"`
	// OldSymrefTarget is the target of a symbolic ref, such as HEAD, before
	// the change, as in "refs/heads/main".
	OldSymrefTarget string `json:"oldSymrefTarget"`
	// NewSymrefTarget is the target of a symbolic ref after the change.
	NewSymrefTarget string `json:"newSymrefTarget"`
	// PusherID is the ID of the Entire account that pushed. It is empty
	// for a push by a service rather than a person.
	PusherID string `json:"pusherId"`
	// At is when the ref changed.
	At Timestamp `json:"eventAt"`
}

// WatchRefs iterates over a repository's ref changes as they happen,
// reconnecting like Changes.WatchEvents. Every connection, including a
// reconnect, starts with a Snapshot of the current refs instead of replaying
// what it missed; rapid changes to one ref arrive at most every 200 ms or so.
// prefixes limits the watch to refs with those name prefixes, such as
// "refs/heads/"; none watches every ref.
func (s *GitService) WatchRefs(ctx context.Context, repoID string, prefixes []string, options *WatchOptions, reqOpts ...RequestOption) iter.Seq2[RefEvent, error] {
	events := watch(ctx, options, func(string) (*EventStream, error) {
		return s.streamRefs(ctx, repoID, prefixes, reqOpts)
	})
	return func(yield func(RefEvent, error) bool) {
		for event, err := range events {
			if err != nil {
				yield(RefEvent{}, err)
				return
			}
			var out RefEvent
			switch event.Type {
			case "snapshot":
				var snapshot struct {
					Refs []RefEntry `json:"refs"`
				}
				err = event.Decode(&snapshot)
				out.Snapshot = append([]RefEntry{}, snapshot.Refs...)
			case "reconnect", "error", "forbidden":
				// The stream's own frames: the connection is ending, and
				// the watch reconnects or reports why it cannot.
				continue
			default:
				out.Change = &RefChange{}
				err = event.Decode(out.Change)
			}
			if err != nil {
				yield(RefEvent{}, fmt.Errorf("entire: decode %s ref event: %w", event.Type, err))
				return
			}
			if !yield(out, nil) {
				return
			}
		}
	}
}

// streamRefs opens one ref event stream.
func (s *GitService) streamRefs(ctx context.Context, repoID string, prefixes []string, reqOpts []RequestOption) (*EventStream, error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/refs/events", repoID)
	for _, prefix := range prefixes {
		req.Query.Add("prefix", prefix)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := s.client.Do(ctx, req, reqOpts...)
	if err != nil {
		return nil, err
	}
	return NewEventStream(resp)
}
