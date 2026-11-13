package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	entire "go.entire.com/entire-sdk-go"
)

// Limits on a check-run upsert.
const (
	// MaxRuns is the most runs one upsert may carry.
	MaxRuns = 5000
	// MaxExternalIDBytes bounds a suite's or run's ExternalID.
	MaxExternalIDBytes = 255
	// MaxOutputTitle bounds Output.Title, in characters.
	MaxOutputTitle = 255
	// MaxOutputSummaryBytes bounds Output.Summary, in UTF-8 bytes.
	MaxOutputSummaryBytes = 65535
	// MaxDeadline is how far ahead a run's DeadlineAt may be.
	MaxDeadline = 24 * time.Hour
)

// Status is a check run's progress.
type Status string

// Statuses.
const (
	Queued     Status = "queued"
	InProgress Status = "in_progress"
	Completed  Status = "completed"
)

// Conclusion is a completed check run's result. Success, Neutral, Skipped,
// and ActionRequired pass; Failure, Cancelled, and TimedOut fail.
type Conclusion string

// Conclusions.
const (
	Success   Conclusion = "success"
	Failure   Conclusion = "failure"
	Neutral   Conclusion = "neutral"
	Cancelled Conclusion = "cancelled"
	Skipped   Conclusion = "skipped"
	TimedOut  Conclusion = "timed_out"
	// ActionRequired marks work waiting on a person, such as a manual
	// approval. It passes, because everything before it finished.
	ActionRequired Conclusion = "action_required"
)

// Outcome says what an upsert did to a run.
type Outcome string

// Outcomes. Treat any other value as "the stored run is in the response;
// whether this write applied is unknown".
const (
	Created   Outcome = "created"
	Updated   Outcome = "updated"
	Unchanged Outcome = "unchanged"
	// IgnoredStale: the stored run is newer than the write, which did not
	// apply.
	IgnoredStale Outcome = "ignored_stale"
)

// Suite is a check suite: the unit customers require, by Key.
type Suite struct {
	// Key identifies the suite across attempts. Keep it stable and
	// readable: customers require checks by it, and a changed key is a new
	// check.
	Key string `json:"key"`
	// Name is shown on the trail.
	Name string `json:"name"`
	// ExternalID is the plugin's ID for this attempt, at most 255 bytes. A
	// new ExternalID starts a new attempt; earlier ones stay as history.
	ExternalID string `json:"external_id"`
	// DetailsURL links to the suite on the plugin's site. It must be https
	// on a host registered for the plugin.
	DetailsURL string `json:"details_url,omitempty"`

	// The suite's own state is optional. Without it, Entire works out the
	// suite's state from its current runs. With it, the reported state is
	// the suite's, whatever its runs say, and writes to it apply in
	// ExternalUpdatedAt order as a run's do. A write that leaves Status
	// empty keeps the stored state.

	// Status is the suite's progress, if the plugin reports it.
	Status Status `json:"-"`
	// Conclusion is the suite's result, required when Status is Completed
	// and empty otherwise.
	Conclusion Conclusion `json:"-"`
	// ExternalUpdatedAt, required with Status, is when this state happened
	// on the plugin's side.
	ExternalUpdatedAt time.Time `json:"-"`
	// StartedAt is when the suite started, if it has.
	StartedAt time.Time `json:"-"`
	// CompletedAt is when the suite completed, if it has.
	CompletedAt time.Time `json:"-"`
}

// MarshalJSON encodes the suite as the API expects, with times in UTC to the
// millisecond and an unreported state left out.
func (s Suite) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key               string     `json:"key"`
		Name              string     `json:"name"`
		ExternalID        string     `json:"external_id"`
		DetailsURL        string     `json:"details_url,omitempty"`
		Status            Status     `json:"status,omitempty"`
		Conclusion        Conclusion `json:"conclusion,omitempty"`
		ExternalUpdatedAt string     `json:"external_updated_at,omitempty"`
		StartedAt         string     `json:"started_at,omitempty"`
		CompletedAt       string     `json:"completed_at,omitempty"`
	}{s.Key, s.Name, s.ExternalID, s.DetailsURL, s.Status, s.Conclusion, optionalTime(s.ExternalUpdatedAt), optionalTime(s.StartedAt), optionalTime(s.CompletedAt)})
}

// Run is one check run in a suite.
type Run struct {
	// Key identifies the run within its suite across attempts.
	Key string
	// Name is shown on the trail.
	Name string
	// ExternalID is the plugin's ID for this attempt, at most 255 bytes.
	ExternalID string
	// Status is the run's progress.
	Status Status
	// Conclusion is the result, required when Status is Completed and
	// empty otherwise.
	Conclusion Conclusion
	// ExternalUpdatedAt, required, is when this state happened on the
	// plugin's side, from the plugin's own records rather than the time of
	// the call. Writes to a run apply in its order, so an older event that
	// arrives late cannot overwrite newer state. Reuse it unchanged when
	// retrying a write.
	ExternalUpdatedAt time.Time
	// StartedAt is when the run started, if it has.
	StartedAt time.Time
	// CompletedAt is when the run completed, if it has.
	CompletedAt time.Time
	// DeadlineAt, at most 24 hours ahead, is when Entire completes an
	// in-progress run as TimedOut. Set it on long-running work: a run that
	// the plugin loses track of otherwise blocks the merge. Zero keeps the
	// stored deadline.
	DeadlineAt time.Time
	// DetailsURL links to the run on the plugin's site. It must be https on
	// a host registered for the plugin.
	DetailsURL string
	// Output is shown on the trail.
	Output *Output
}

// Output is what a run shows on the trail.
type Output struct {
	// Title is up to 255 characters.
	Title string `json:"title,omitempty"`
	// Summary is Markdown, up to 65,535 UTF-8 bytes.
	Summary string `json:"summary,omitempty"`
}

// apiTime formats t as RFC 3339 UTC with milliseconds, which is how the API
// reads it.
func apiTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// optionalTime is apiTime, or empty for the zero time.
func optionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return apiTime(t)
}

// MarshalJSON encodes the run as the API expects, with times in UTC to the
// millisecond and zero times left out.
func (r Run) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key               string     `json:"key"`
		Name              string     `json:"name"`
		ExternalID        string     `json:"external_id"`
		Status            Status     `json:"status"`
		Conclusion        Conclusion `json:"conclusion,omitempty"`
		ExternalUpdatedAt string     `json:"external_updated_at"`
		StartedAt         string     `json:"started_at,omitempty"`
		CompletedAt       string     `json:"completed_at,omitempty"`
		DeadlineAt        string     `json:"deadline_at,omitempty"`
		DetailsURL        string     `json:"details_url,omitempty"`
		Output            *Output    `json:"output,omitempty"`
	}{r.Key, r.Name, r.ExternalID, r.Status, r.Conclusion, apiTime(r.ExternalUpdatedAt), optionalTime(r.StartedAt), optionalTime(r.CompletedAt), optionalTime(r.DeadlineAt), r.DetailsURL, r.Output})
}

// UpsertResult is the stored state after an upsert.
type UpsertResult struct {
	// Suite is the suite attempt written.
	Suite SuiteRef `json:"check_suite"`
	// Results has one entry per run, in request order.
	Results []RunResult `json:"results"`
}

// SuiteRef identifies a stored suite attempt.
type SuiteRef struct {
	// ID is Entire's ID for the suite attempt.
	ID string `json:"id"`
	// Key is the suite's key.
	Key string `json:"key"`
	// ExternalID is the plugin's ID for the attempt.
	ExternalID string `json:"external_id"`
}

// RunResult is what an upsert did to one run.
type RunResult struct {
	// Run is the stored run, whether or not this write applied.
	Run RunRef `json:"check_run"`
	// Outcome says whether this write applied.
	Outcome Outcome `json:"outcome"`
}

// RunRef is a stored run's identity and state.
type RunRef struct {
	// ID is Entire's ID for the run.
	ID string `json:"id"`
	// Key is the run's key.
	Key string `json:"key"`
	// Status is the stored status.
	Status Status `json:"status"`
	// Conclusion is the stored conclusion, empty until completed.
	Conclusion Conclusion `json:"conclusion"`
}

// StoredRun is one of the plugin's current runs on a commit. Times the
// plugin never sent are zero.
type StoredRun struct {
	// ID is Entire's ID for the run.
	ID string `json:"id"`
	// SuiteID is Entire's ID for the run's suite attempt.
	SuiteID string `json:"suite_id"`
	// SuiteKey is the key of the run's suite.
	SuiteKey          string           `json:"suite_key"`
	Key               string           `json:"key"`
	Name              string           `json:"name"`
	ExternalID        string           `json:"external_id"`
	Status            Status           `json:"status"`
	Conclusion        Conclusion       `json:"conclusion"`
	ExternalUpdatedAt entire.Timestamp `json:"external_updated_at"`
	StartedAt         entire.Timestamp `json:"started_at"`
	CompletedAt       entire.Timestamp `json:"completed_at"`
	DeadlineAt        entire.Timestamp `json:"deadline_at"`
	DetailsURL        string           `json:"details_url"`
	Output            *Output          `json:"output"`
}

// StoredSuite is one of the plugin's suites on a commit, with the status and
// conclusion that Entire worked out from its current runs. The merge reads
// these.
type StoredSuite struct {
	ID         string     `json:"id"`
	Key        string     `json:"key"`
	Name       string     `json:"name"`
	ExternalID string     `json:"external_id"`
	Status     Status     `json:"status"`
	Conclusion Conclusion `json:"conclusion"`
	DetailsURL string     `json:"details_url"`
}

// Checks writes and reads the plugin's check runs. Every suite and run
// belongs to the plugin that wrote it: other plugins can neither read nor
// change them, and the reads return only the plugin's own.
//
// How Entire orders writes, and what the merge reads:
//
//   - The current attempt of a suite is the one whose runs carry the newest
//     ExternalUpdatedAt, and the current run for a key is the newest one.
//   - A write applies when its ExternalUpdatedAt is at or after the stored
//     run's. Equal times apply, so retrying the same write is safe.
//   - A queued or in-progress write cannot reopen a completed run at the
//     same time; a newer one can, which is how a retried run comes back.
//   - A Cancelled result never replaces a passing one, whatever its time.
//   - A suite passes when all its current runs pass, unless it reports its
//     own state, which then decides. If partial results, such as the jobs
//     that finished first, could look like a complete pass, report the
//     suite's state, and keep it in progress until the whole build
//     finishes.
//
// Checks needs Permissions.Checks: Write. Each call uses a token for its
// repository that carries only that permission, so Config.Permissions may
// name fewer permissions than the plugin registered.
type Checks struct{ inst *Installation }

var checksWrite = Permissions{Checks: Write}

// UpsertRequest is one check-run write: a suite and its runs on a commit of
// a branch.
type UpsertRequest struct {
	// Branch is the branch the commit was built on, such as "main". A
	// leading "refs/heads/" is removed.
	Branch string
	// Suite is the suite the runs belong to.
	Suite Suite
	// Runs are the runs to write: 1 to 5,000, with no two sharing a key and
	// external ID.
	Runs []Run
}

// Upsert writes a suite and its runs on a commit, whenever their state
// changes. It is idempotent, and atomic: the whole write applies or none of
// it. Writes to a run apply in ExternalUpdatedAt order, so a stale write
// returns the stored run with the outcome IgnoredStale.
//
// The SDK checks the write as the API does before sending it, sends it to
// the repository's processing primary, retries failures with the same body,
// and when Entire answers that the repository moved (wrong_cell), lists the
// repositories again and retries at the new host. repoID is a public
// repository ID, such as Repository.ID.
func (c *Checks) Upsert(ctx context.Context, repoID, sha string, req UpsertRequest) (*entire.Response[UpsertResult], error) {
	branch := strings.TrimPrefix(req.Branch, "refs/heads/")
	if err := validateUpsert(repoID, sha, branch, req.Suite, req.Runs, c.inst.app.now()); err != nil {
		return nil, err
	}
	body := struct {
		Branch string `json:"branch"`
		Suite  Suite  `json:"check_suite"`
		Runs   []Run  `json:"check_runs"`
	}{branch, req.Suite, req.Runs}
	return send[UpsertResult](ctx, c.inst, "v1", repoID, checksWrite, func(client *entire.Client) *entire.Request {
		r := client.NewRequest(http.MethodPost, checkPath(repoID, sha, "check-runs"), body)
		// The upsert is idempotent, so a failed attempt may be retried.
		r.Idempotent = true
		return r
	})
}

// ListRunsOptions filters ListRuns.
type ListRunsOptions struct {
	// SuiteKey limits the runs to one suite.
	SuiteKey string
}

// ListRuns returns the plugin's current runs on a commit, without
// superseded attempts.
func (c *Checks) ListRuns(ctx context.Context, repoID, sha string, opts *ListRunsOptions) (*entire.Response[[]StoredRun], error) {
	if err := validateCommit(repoID, sha); err != nil {
		return nil, err
	}
	resp, err := send[struct {
		Runs []StoredRun `json:"check_runs"`
	}](ctx, c.inst, "v1", repoID, checksWrite, func(client *entire.Client) *entire.Request {
		r := client.NewRequest(http.MethodGet, checkPath(repoID, sha, "check-runs"), nil)
		if opts != nil && opts.SuiteKey != "" {
			r.Query.Set("suite_key", opts.SuiteKey)
		}
		return r
	})
	return unwrap(resp, err, func(v struct {
		Runs []StoredRun `json:"check_runs"`
	}) []StoredRun {
		return v.Runs
	})
}

// ListSuites returns the plugin's suites on a commit, with the status and
// conclusion the merge reads.
func (c *Checks) ListSuites(ctx context.Context, repoID, sha string) (*entire.Response[[]StoredSuite], error) {
	if err := validateCommit(repoID, sha); err != nil {
		return nil, err
	}
	resp, err := send[struct {
		Suites []StoredSuite `json:"check_suites"`
	}](ctx, c.inst, "v1", repoID, checksWrite, func(client *entire.Client) *entire.Request {
		return client.NewRequest(http.MethodGet, checkPath(repoID, sha, "check-suites"), nil)
	})
	return unwrap(resp, err, func(v struct {
		Suites []StoredSuite `json:"check_suites"`
	}) []StoredSuite {
		return v.Suites
	})
}

func unwrap[W, T any](resp *entire.Response[W], err error, data func(W) T) (*entire.Response[T], error) {
	if resp == nil {
		return nil, err
	}
	return &entire.Response[T]{Data: data(resp.Data), StatusCode: resp.StatusCode, Header: resp.Header, HTTPResponse: resp.HTTPResponse}, err
}

func checkPath(repoID, sha, resource string) string {
	return "/repos/" + url.PathEscape(repoID) + "/commits/" + url.PathEscape(sha) + "/" + resource
}

// send calls a check-run or deployment endpoint of a repository, under an
// API version such as "v1", with a token for the repository that carries
// permissions, at the repository's processing primary, and once more at its
// new primary after a wrong_cell answer.
func send[T any](ctx context.Context, inst *Installation, version, repoID string, permissions Permissions, build func(*entire.Client) *entire.Request) (*entire.Response[T], error) {
	repo, err := inst.Repository(ctx, repoID)
	if err != nil {
		return nil, err
	}
	token, err := inst.RepositoryToken(ctx, repoID, permissions)
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		apiURL := repo.PrimaryAPIURL()
		if apiURL == "" {
			return nil, fmt.Errorf("%w: %s", ErrRepositoryNotReady, repoID)
		}
		client, err := inst.apiClient(apiURL, version)
		if err != nil {
			return nil, err
		}
		resp, err := entire.Call[T](ctx, client, build(client), entire.WithHeader("Authorization", "Bearer "+token.Value))
		if attempt > 0 || !entire.IsWrongCell(err) {
			return resp, err
		}
		repos, listErr := inst.Repositories(ctx)
		if listErr != nil {
			return resp, err
		}
		moved := false
		for _, r := range repos {
			if r.ID == repoID && r.PrimaryAPIURL() != apiURL {
				repo, moved = r, true
			}
		}
		if !moved {
			return resp, err
		}
	}
}

func validateCommit(repoID, sha string) error {
	if !strings.HasPrefix(repoID, "repo_") || len(repoID) == len("repo_") {
		return fmt.Errorf("plugin: %q is not a public repository ID (repo_…)", repoID)
	}
	if !isSHA(sha) {
		return fmt.Errorf("plugin: %q is not a full commit SHA (40 or 64 hex digits)", sha)
	}
	return nil
}

func isSHA(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func validateUpsert(repoID, sha, branch string, suite Suite, runs []Run, now time.Time) error {
	if err := validateCommit(repoID, sha); err != nil {
		return err
	}
	if branch == "" {
		return errors.New("plugin: branch is required")
	}
	if err := validateIdentity("check suite", suite.Key, suite.Name, suite.ExternalID); err != nil {
		return err
	}
	if err := validateDetailsURL(suite.DetailsURL); err != nil {
		return fmt.Errorf("plugin: check suite: %w", err)
	}
	if err := validateSuiteState(suite); err != nil {
		return fmt.Errorf("plugin: check suite: %w", err)
	}
	if len(runs) == 0 || len(runs) > MaxRuns {
		return fmt.Errorf("plugin: an upsert needs 1 to %d runs, not %d", MaxRuns, len(runs))
	}
	type identity struct{ key, externalID string }
	seen := make(map[identity]bool, len(runs))
	for n, r := range runs {
		fail := func(format string, args ...any) error {
			return fmt.Errorf("plugin: check run %d (%q): %s", n, r.Key, fmt.Sprintf(format, args...))
		}
		if err := validateIdentity(fmt.Sprintf("check run %d", n), r.Key, r.Name, r.ExternalID); err != nil {
			return err
		}
		id := identity{r.Key, r.ExternalID}
		if seen[id] {
			return fail("repeats key and external ID %q", r.ExternalID)
		}
		seen[id] = true
		switch {
		case r.Status != Queued && r.Status != InProgress && r.Status != Completed:
			return fail("status %q is not queued, in_progress, or completed", r.Status)
		case r.Status == Completed && r.Conclusion == "":
			return fail("a completed run needs a conclusion")
		case r.Status == Completed && !knownConclusion(r.Conclusion):
			return fail("conclusion %q is not one the API accepts", r.Conclusion)
		case r.Status != Completed && r.Conclusion != "":
			return fail("only a completed run has a conclusion")
		case r.ExternalUpdatedAt.IsZero():
			return fail("external updated time is required; set it from the plugin's own records")
		case !r.StartedAt.IsZero() && !r.CompletedAt.IsZero() && r.CompletedAt.Before(r.StartedAt):
			return fail("completed before it started")
		case !r.DeadlineAt.IsZero() && r.DeadlineAt.After(now.Add(MaxDeadline)):
			return fail("deadline is more than 24 hours ahead")
		}
		if err := validateDetailsURL(r.DetailsURL); err != nil {
			return fail("%v", err)
		}
		if r.Output != nil {
			if utf8.RuneCountInString(r.Output.Title) > MaxOutputTitle {
				return fail("output title exceeds %d characters", MaxOutputTitle)
			}
			if len(r.Output.Summary) > MaxOutputSummaryBytes {
				return fail("output summary exceeds %d bytes", MaxOutputSummaryBytes)
			}
		}
	}
	return nil
}

// validateSuiteState checks a suite's reported state, if it has one, as the
// API does.
func validateSuiteState(s Suite) error {
	if s.Status == "" && s.Conclusion == "" && s.ExternalUpdatedAt.IsZero() && s.StartedAt.IsZero() && s.CompletedAt.IsZero() {
		return nil
	}
	switch {
	case s.Status == "" || s.ExternalUpdatedAt.IsZero():
		return errors.New("status and external updated time must be set together")
	case s.Status != Queued && s.Status != InProgress && s.Status != Completed:
		return fmt.Errorf("status %q is not queued, in_progress, or completed", s.Status)
	case s.Status == Completed && s.Conclusion == "":
		return errors.New("a completed suite needs a conclusion")
	case s.Status == Completed && !knownConclusion(s.Conclusion):
		return fmt.Errorf("conclusion %q is not one the API accepts", s.Conclusion)
	case s.Status != Completed && s.Conclusion != "":
		return errors.New("only a completed suite has a conclusion")
	case !s.StartedAt.IsZero() && !s.CompletedAt.IsZero() && s.CompletedAt.Before(s.StartedAt):
		return errors.New("completed before it started")
	}
	return nil
}

func knownConclusion(c Conclusion) bool {
	switch c {
	case Success, Failure, Neutral, Cancelled, Skipped, TimedOut, ActionRequired:
		return true
	}
	return false
}

func validateIdentity(what, key, name, externalID string) error {
	switch {
	case key == "":
		return fmt.Errorf("plugin: %s: key is required", what)
	case name == "":
		return fmt.Errorf("plugin: %s: name is required", what)
	case externalID == "":
		return fmt.Errorf("plugin: %s: external ID is required", what)
	case len(externalID) > MaxExternalIDBytes:
		return fmt.Errorf("plugin: %s: external ID exceeds %d bytes", what, MaxExternalIDBytes)
	}
	return nil
}

func validateDetailsURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("details URL %q is not an https URL", raw)
	}
	return nil
}
