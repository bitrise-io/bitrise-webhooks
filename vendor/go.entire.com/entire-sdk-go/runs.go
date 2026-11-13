package entire

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
)

// RunsService starts and reads agent runs. A run is one review of one
// Change by one agent, pinned to the agent's version, its Environment, and
// the Change's commits when it was admitted.
//
// Runs are reachable two ways. The project methods take a project path, such
// as "gh/acme", and need project read access or pull on one of the
// project's repositories. The InRepo methods take a repository ID and need
// only pull on that repository. Either way, reading a run needs pull on its
// repository, and starting, retrying, or canceling one needs push.
//
// These are the runs of a project's agents. A repository's runner runs,
// from runner files in the repository, are another API.
//
// These endpoints are not yet part of Entire's published API and may change
// without a major SDK release.
type RunsService service

// RunStatus is a run's status.
type RunStatus string

// Run statuses.
const (
	// RunQueued is waiting for a sandbox, or, with WaitingFor set to
	// RunWaitingForSelfHostedRunner, for a self-hosted runner to claim it.
	RunQueued    RunStatus = "queued"
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunTimedOut  RunStatus = "timed_out"
	RunCanceled  RunStatus = "canceled"
)

// RunWaitingFor is what a queued run waits on besides a sandbox.
type RunWaitingFor string

// What a queued run waits for.
const (
	// RunWaitingForSelfHostedRunner: the run's Environment runs on
	// RunsOnSelfHosted, and no runner has claimed it yet.
	RunWaitingForSelfHostedRunner RunWaitingFor = "self_hosted_runner"
)

// RunTriggerKind is why a run was admitted.
type RunTriggerKind string

// Run trigger kinds.
const (
	// RunTriggerPush: the agent's push trigger.
	RunTriggerPush RunTriggerKind = "push"
	// RunTriggerManual: RunsService.Start or StartInRepo.
	RunTriggerManual RunTriggerKind = "manual"
	// RunTriggerRetry: RunsService.Retry or RetryInRepo.
	RunTriggerRetry RunTriggerKind = "retry"
)

// RunDispatchState is how far the API got in handing a run to a worker.
type RunDispatchState string

// Run dispatch states.
const (
	RunDispatchPending   RunDispatchState = "pending"
	RunDispatchPublished RunDispatchState = "published"
	RunDispatchRefused   RunDispatchState = "refused"
	RunDispatchAbandoned RunDispatchState = "abandoned"
	// RunDispatchParked: a self-hosted run waits for a runner.
	RunDispatchParked RunDispatchState = "parked"
)

// Values of RunError.Code, set when billing refused a run.
const (
	RunErrorBillingTrialEnded      = "billing_trial_ended"
	RunErrorBillingCreditExhausted = "billing_credit_exhausted"
	RunErrorBillingOverLimit       = "billing_over_limit"
	RunErrorBillingBlocked         = "billing_blocked"
	RunErrorBillingNotEnrolled     = "billing_not_enrolled"
	RunErrorBillingNoPaidBalance   = "billing_no_paid_balance"
)

// RunReasonUnclaimedSelfHosted begins the RunError.Message of a self-hosted
// run that timed out because no runner claimed it within its Environment's
// ClaimTimeoutMS. Start a runner and retry the run.
const RunReasonUnclaimedSelfHosted = "reaper_unclaimed_self_hosted"

// RunError is why a run failed or timed out, or why Entire canceled it.
type RunError struct {
	// Code is one of the RunErrorBilling constants when billing refused
	// the run, and empty otherwise.
	Code string `json:"code"`
	// Message describes the failure.
	Message string `json:"message"`
}

// RunAgent is the agent a run pinned.
type RunAgent struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	// Name is the agent's name as the run pinned it.
	Name string `json:"name"`
}

// RunVersionRef is a versioned record a run pinned, such as its
// Environment.
type RunVersionRef struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

// RunTrigger is why a run was admitted.
type RunTrigger struct {
	Kind RunTriggerKind `json:"kind"`
	// ID is the agent trigger's ID for a push run; empty otherwise.
	ID string `json:"id"`
}

// RunSelfHosted records who ran a run in a self-hosted Environment. Each
// field is empty until the runner claims the run or reports.
type RunSelfHosted struct {
	// Runner is the service account that claimed the run.
	Runner string `json:"runner"`
	// Harness and Model are what the runner reported using. Entire does
	// not verify them.
	Harness string `json:"harness"`
	Model   string `json:"model"`
}

// RunModelUsage is token usage the agent reported.
type RunModelUsage struct {
	// Model is the model the tokens are attributed to; empty for a total
	// across several models, or when attribution is unavailable.
	Model               string `json:"model"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	CacheReadTokens     int64  `json:"cache_read_tokens"`
}

// RunBillingSubject is who pays for a run.
type RunBillingSubject struct {
	// Type is "org" or "account".
	Type string `json:"type"`
	ID   string `json:"id"`
}

// RunSummary is a run as the list methods return it. ListInRepo leaves
// ProjectID, RepositoryID, BillingSubject, DispatchState, Environment, and
// Actor empty.
type RunSummary struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	RepositoryID string    `json:"repository_id"`
	Status       RunStatus `json:"status"`
	// Error is why the run failed, timed out, or was canceled by Entire;
	// nil otherwise, including for a run canceled with Cancel.
	Error *RunError `json:"error"`
	// BillingSubject is who pays; nil for runs launched before it was
	// recorded, and for a caller who reads only the project's
	// repositories.
	BillingSubject *RunBillingSubject `json:"billing_subject"`
	DispatchState  RunDispatchState   `json:"dispatch_state"`
	// WaitingFor is set while a queued run waits on something besides a
	// sandbox.
	WaitingFor RunWaitingFor `json:"waiting_for"`
	// SelfHosted is set only for a run in a self-hosted Environment.
	SelfHosted  *RunSelfHosted `json:"self_hosted"`
	Agent       RunAgent       `json:"agent"`
	Environment RunVersionRef  `json:"environment"`
	Trigger     RunTrigger     `json:"trigger"`
	// TrailID and ChangeID are the Change the run reviews, and Branch its
	// branch as the run pinned it.
	TrailID  string `json:"trail_id"`
	ChangeID string `json:"change_id"`
	Branch   string `json:"branch"`
	// Actor is who started the run.
	Actor ConfigActor `json:"actor"`
	// Attempt counts from 1; RetryOf is the run a retry repeats.
	Attempt int    `json:"attempt"`
	RetryOf string `json:"retry_of"`
	// UsageByModel is the token usage reported so far; nil before the
	// first report.
	UsageByModel []RunModelUsage `json:"usage_by_model"`
	CreatedAt    Timestamp       `json:"created_at"`
	UpdatedAt    Timestamp       `json:"updated_at"`
}

// Run is one run of an agent, with what it pinned and what it produced.
type Run struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	RepositoryID string    `json:"repository_id"`
	Status       RunStatus `json:"status"`
	// Error is as on RunSummary.
	Error *RunError `json:"error"`
	// Diagnostic is the redacted tail of a failed run's process output, at
	// most 4 KiB. It is empty without push on the repository or
	// canManageAgents, and may contain repository content.
	Diagnostic     string             `json:"diagnostic"`
	BillingSubject *RunBillingSubject `json:"billing_subject"`
	Agent          RunAgent           `json:"agent"`
	Environment    RunVersionRef      `json:"environment"`
	Trigger        RunTrigger         `json:"trigger"`
	Target         RunTarget          `json:"target"`
	Runtime        RunRuntime         `json:"runtime"`
	// Preset is the preset the run ran, or nil for a custom agent's run.
	Preset  *RunPreset  `json:"preset"`
	Actor   ConfigActor `json:"actor"`
	Attempt int         `json:"attempt"`
	RetryOf string      `json:"retry_of"`
	// Dispatch is how far the API got in handing the run to a worker.
	Dispatch   *RunDispatch   `json:"dispatch"`
	WaitingFor RunWaitingFor  `json:"waiting_for"`
	SelfHosted *RunSelfHosted `json:"self_hosted"`
	// Result is the result the run stored, in its raw form.
	Result *RunResult `json:"result"`
	// Mode is the review contract the run pinned.
	Mode RunMode `json:"mode"`
	// ReviewChangesResult is a completed run's review; nil otherwise.
	ReviewChangesResult *RunReviewResult `json:"review_changes_result"`
	// Output is the agent's captured final answer, or an excerpt of it
	// until the run has an accepted result; nil when none was captured.
	Output       *RunOutput      `json:"output"`
	UsageByModel []RunModelUsage `json:"usage_by_model"`
	// Events are the run's raw events; empty without push on the
	// repository or canManageAgents.
	Events    []RunEvent `json:"events"`
	CreatedAt Timestamp  `json:"created_at"`
	UpdatedAt Timestamp  `json:"updated_at"`
	// Replayed is true when Start or Retry returned the run an earlier
	// request with the same Idempotency-Key started.
	Replayed bool `json:"replayed"`
}

// RunTarget is the Change a run reviews, at the commits it pinned.
type RunTarget struct {
	RepositoryID string `json:"repository_id"`
	TrailID      string `json:"trail_id"`
	ChangeID     string `json:"change_id"`
	// Ref is the Change's branch.
	Ref     string `json:"ref"`
	HeadSHA string `json:"head_sha"`
	BaseRef string `json:"base_ref"`
	BaseSHA string `json:"base_sha"`
}

// RunRuntime is what a run ran with.
type RunRuntime struct {
	Harness         AgentHarness    `json:"harness"`
	Model           string          `json:"model"`
	ReasoningEffort ReasoningEffort `json:"reasoning_effort"`
	TimeoutMS       int             `json:"timeout_ms"`
	Checkout        string          `json:"checkout"`
	GitAccess       string          `json:"git_access"`
	AutoStopMinutes int             `json:"auto_stop_minutes"`
}

// RunPreset is the preset a run ran, as it pinned it.
type RunPreset struct {
	ID string `json:"id"`
	// Version and SHA256 are zero and empty for runs from before presets
	// were versioned.
	Version int    `json:"version"`
	SHA256  string `json:"sha256"`
}

// RunDispatch is how far the API got in handing a run to a worker.
type RunDispatch struct {
	State         RunDispatchState `json:"state"`
	Attempts      int              `json:"attempts"`
	NextAttemptAt Timestamp        `json:"next_attempt_at"`
	PublishedAt   Timestamp        `json:"published_at"`
}

// RunResult is the result a run stored.
type RunResult struct {
	ResultType string          `json:"result_type"`
	Data       json.RawMessage `json:"data"`
	CreatedAt  Timestamp       `json:"created_at"`
}

// RunMode is the review contract a run pinned.
type RunMode struct {
	// Kind is "review".
	Kind            string `json:"kind"`
	ContractVersion string `json:"contract_version"`
	// OutputMode is how the harness was asked for its result, such as
	// "claude_json_schema".
	OutputMode string `json:"output_mode"`
	// OutputSchemaSHA256 hashes the output schema the harness was given;
	// Snapshot returns the schema.
	OutputSchemaSHA256 string      `json:"output_schema_sha256"`
	Signal             AgentSignal `json:"signal"`
	Reviewed           struct {
		HeadSHA string `json:"head_sha"`
		// BaseSHA is the base branch's commit at admission, and
		// DiffBaseSHA the merge base the diff was taken against.
		BaseSHA     string `json:"base_sha"`
		DiffBaseSHA string `json:"diff_base_sha"`
	} `json:"reviewed"`
}

// RunReviewResult is a completed run's review.
type RunReviewResult struct {
	// Summary is the review as a whole.
	Summary string `json:"summary"`
	// Signal is the reading, with its descriptor.
	Signal struct {
		AgentSignal
		// Value is a boolean, an integer from 0 to 100, or "small",
		// "medium", or "large", by the signal's ValueType.
		Value     json.RawMessage `json:"value"`
		Rationale string          `json:"rationale"`
	} `json:"signal"`
	Findings []RunFinding `json:"findings"`
	// PriorFindings are the run's verdicts on its agent's earlier open
	// findings.
	PriorFindings []RunPriorFinding `json:"prior_findings"`
	// ReviewID is the trail review that published the findings.
	ReviewID string `json:"review_id"`
}

// RunFinding is one finding of a review.
type RunFinding struct {
	Body string `json:"body"`
	// Severity is "high", "medium", or "low".
	Severity   string  `json:"severity"`
	Confidence float64 `json:"confidence"`
	Location   struct {
		// Granularity is "line", "range", or "file".
		Granularity string `json:"granularity"`
		FilePath    string `json:"file_path"`
		StartLine   int    `json:"start_line"`
		EndLine     int    `json:"end_line"`
	} `json:"location"`
}

// RunPriorFinding is a run's verdict on one of its agent's earlier
// findings.
type RunPriorFinding struct {
	FindingID string `json:"finding_id"`
	// Verdict is "fixed", "still_present", or "no_longer_applies".
	Verdict  string `json:"verdict"`
	Evidence string `json:"evidence"`
	// Outcome is what Entire did with the finding, such as "resolved";
	// empty once the finding is deleted.
	Outcome string `json:"outcome"`
}

// RunOutput is an agent's captured final answer.
type RunOutput struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}

// RunEvent is one raw event of a run.
type RunEvent struct {
	Kind      string    `json:"kind"`
	Type      string    `json:"type"`
	Detail    string    `json:"detail"`
	CreatedAt Timestamp `json:"created_at"`
}

// RunStartRequest names the Change a run reviews.
type RunStartRequest struct {
	// TrailID is a trail of the project, and ChangeID one of its Changes.
	// The agent must select the Change's repository.
	TrailID  string
	ChangeID string
	// HeadSHA, when set, refuses the run with 409 Conflict if the Change's
	// head is no longer this commit.
	HeadSHA string
}

// runStartBody is the wire form of a run start.
type runStartBody struct {
	AgentID string `json:"agent_id,omitempty"`
	Target  struct {
		TrailID  string `json:"trail_id"`
		ChangeID string `json:"change_id"`
	} `json:"target"`
	HeadSHA string `json:"head_sha,omitempty"`
}

func (r RunStartRequest) body(agentID string) runStartBody {
	b := runStartBody{AgentID: agentID, HeadSHA: r.HeadSHA}
	b.Target.TrailID, b.Target.ChangeID = r.TrailID, r.ChangeID
	return b
}

// RunListOptions filters RunsService.List and ListInRepo. The filters
// combine.
type RunListOptions struct {
	// AgentID selects one agent's runs.
	AgentID string `url:"agent_id"`
	// RepositoryID selects one repository's runs. ListInRepo ignores it.
	RepositoryID string `url:"repository_id"`
	// TrailID and ChangeID select the runs on a trail's Changes, or on one
	// Change.
	TrailID  string `url:"trail_id"`
	ChangeID string `url:"change_id"`
	// Branch selects runs on a branch.
	Branch string `url:"branch"`
	// Status selects runs in one status.
	Status RunStatus `url:"status"`
	ListOptions
}

// Start runs an agent on one of a trail's Changes. The run pins the agent's
// current version, its Environment, and the Change's commits, and is queued:
// poll it with Get. A paused agent runs only for a caller with
// canManageAgents. It needs push on the Change's repository. The request
// carries a new Idempotency-Key, so a RetryPolicy may resend it without
// starting a second run; pass WithHeader("Idempotency-Key", key) to retry
// with your own key across calls.
func (s *RunsService) Start(ctx context.Context, projectPath, agentID string, body RunStartRequest, reqOpts ...RequestOption) (*Response[Run], error) {
	req := projectRequest(http.MethodPost, projectPath, body.body(""), "/agents/%s/runs", agentID)
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	return Call[Run](ctx, s.client, req, reqOpts...)
}

// List returns one page of the project's runs, newest first, in the
// repositories the caller can pull.
func (s *RunsService) List(ctx context.Context, projectPath string, opts *RunListOptions, reqOpts ...RequestOption) (*Response[[]RunSummary], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/runs")
	addQuery(req.Query, opts)
	Deref(opts).ListOptions.add(req.Query, "per_page", "cursor")
	return listCursor[RunSummary](ctx, s.client, req, reqOpts)
}

// ListAll iterates over every run that List returns, page by page.
func (s *RunsService) ListAll(ctx context.Context, projectPath string, opts *RunListOptions, reqOpts ...RequestOption) iter.Seq2[RunSummary, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]RunSummary], error) {
		p := o
		p.PageToken = token
		return s.List(ctx, projectPath, &p, reqOpts...)
	})
}

// Get returns a run.
func (s *RunsService) Get(ctx context.Context, projectPath, runID string, reqOpts ...RequestOption) (*Response[Run], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/runs/%s", runID)
	return Call[Run](ctx, s.client, req, reqOpts...)
}

// Snapshot returns the immutable snapshot a run was admitted with: every
// pinned spec, the resolved target, the rendered prompt, and the runtime.
// Without canManageAgents it holds only what the run records of itself.
func (s *RunsService) Snapshot(ctx context.Context, projectPath, runID string, reqOpts ...RequestOption) (*Response[json.RawMessage], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/runs/%s/snapshot", runID)
	return Call[json.RawMessage](ctx, s.client, req, reqOpts...)
}

// Retry starts the next attempt of a failed, timed-out, or canceled run
// from its snapshot. The request carries a new Idempotency-Key, as Start
// does.
func (s *RunsService) Retry(ctx context.Context, projectPath, runID string, reqOpts ...RequestOption) (*Response[Run], error) {
	req := projectRequest(http.MethodPost, projectPath, nil, "/runs/%s/retry", runID)
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	return Call[Run](ctx, s.client, req, reqOpts...)
}

// Cancel cancels a queued or running run and returns it; a finished run is
// returned as it ended. Canceling a run someone else or a push started
// needs canManageAgents.
func (s *RunsService) Cancel(ctx context.Context, projectPath, runID string, reqOpts ...RequestOption) (*Response[Run], error) {
	req := projectRequest(http.MethodPost, projectPath, nil, "/runs/%s/cancel", runID)
	req.Idempotent = true
	return Call[Run](ctx, s.client, req, reqOpts...)
}

// StartInRepo runs agentID, an agent of the repository's project, on a
// Change in the repository, as Start does. A Change in another repository
// fails with 422 Unprocessable Entity.
func (s *RunsService) StartInRepo(ctx context.Context, repoID, agentID string, body RunStartRequest, reqOpts ...RequestOption) (*Response[Run], error) {
	req := repoRequest(http.MethodPost, repoID, body.body(agentID), "/repos/%s/runs", repoID)
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	return Call[Run](ctx, s.client, req, reqOpts...)
}

// ListInRepo returns one page of the repository's agent runs, newest first,
// archived agents' included.
func (s *RunsService) ListInRepo(ctx context.Context, repoID string, opts *RunListOptions, reqOpts ...RequestOption) (*Response[[]RunSummary], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/runs", repoID)
	if opts != nil {
		o := *opts
		o.RepositoryID = ""
		addQuery(req.Query, &o)
		o.ListOptions.add(req.Query, "per_page", "cursor")
	}
	return listCursor[RunSummary](ctx, s.client, req, reqOpts)
}

// ListInRepoAll iterates over every run that ListInRepo returns, page by
// page.
func (s *RunsService) ListInRepoAll(ctx context.Context, repoID string, opts *RunListOptions, reqOpts ...RequestOption) iter.Seq2[RunSummary, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]RunSummary], error) {
		p := o
		p.PageToken = token
		return s.ListInRepo(ctx, repoID, &p, reqOpts...)
	})
}

// GetInRepo returns one of the repository's runs.
func (s *RunsService) GetInRepo(ctx context.Context, repoID, runID string, reqOpts ...RequestOption) (*Response[Run], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/runs/%s", repoID, runID)
	return Call[Run](ctx, s.client, req, reqOpts...)
}

// SnapshotInRepo returns the snapshot of one of the repository's runs, as
// Snapshot does.
func (s *RunsService) SnapshotInRepo(ctx context.Context, repoID, runID string, reqOpts ...RequestOption) (*Response[json.RawMessage], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/runs/%s/snapshot", repoID, runID)
	return Call[json.RawMessage](ctx, s.client, req, reqOpts...)
}

// RetryInRepo retries one of the repository's runs, as Retry does.
func (s *RunsService) RetryInRepo(ctx context.Context, repoID, runID string, reqOpts ...RequestOption) (*Response[Run], error) {
	req := repoRequest(http.MethodPost, repoID, nil, "/repos/%s/runs/%s/retry", repoID, runID)
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	return Call[Run](ctx, s.client, req, reqOpts...)
}

// CancelInRepo cancels one of the repository's runs, as Cancel does.
func (s *RunsService) CancelInRepo(ctx context.Context, repoID, runID string, reqOpts ...RequestOption) (*Response[Run], error) {
	req := repoRequest(http.MethodPost, repoID, nil, "/repos/%s/runs/%s/cancel", repoID, runID)
	req.Idempotent = true
	return Call[Run](ctx, s.client, req, reqOpts...)
}

// RunnerAdmission says whether Entire admits runs for a repository or
// project.
type RunnerAdmission struct {
	// RunsPaused reports that runs are refused.
	RunsPaused bool `json:"runs_paused"`
	// PausedBy is "billing" or "operator" while runs are paused, and empty
	// otherwise. More values may be added.
	PausedBy string `json:"paused_by"`
	// BillingSubject is who pays for the runs; nil when Core names none.
	BillingSubject *RunBillingSubject `json:"billing_subject"`
}

// RepoAdmission reports whether the repository's runs are paused. It needs
// read access to the repository.
func (s *RunsService) RepoAdmission(ctx context.Context, repoID string, reqOpts ...RequestOption) (*Response[RunnerAdmission], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/runner-admission", repoID)
	return Call[RunnerAdmission](ctx, s.client, req, reqOpts...)
}

// ProjectAdmission reports whether the project's runs are paused. It needs
// read access to the project.
func (s *RunsService) ProjectAdmission(ctx context.Context, projectPath string, reqOpts ...RequestOption) (*Response[RunnerAdmission], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/runner-status")
	return Call[RunnerAdmission](ctx, s.client, req, reqOpts...)
}
