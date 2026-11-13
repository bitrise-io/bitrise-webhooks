package entire

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
)

// RunnerService is the API of a self-hosted runner: a machine that reviews
// the runs of a project's self-hosted agents with its own harness and model
// login, and posts the results. Its methods need a service account token,
// such as a client made from a service account secret (ent_svc_…), whose
// account a project admin bound to the agents' Environment with
// EnvironmentsService.BindRunner and granted read on their repositories.
//
// A runner lists the runs it may claim with ListJobs, claims one with Claim,
// which returns the prompt and how to fetch the code, and finishes it with
// Complete or Fail before the claim's DeadlineAt. Entire never pushes work to
// a runner; it decides when a run exists, and the runner only picks up runs
// that do. Use one service account per machine: a claim by an account that
// already holds a run returns that run again.
//
// A project's runs live in the cell that processes its repositories. ListJobs
// asks the caller's home cell unless told otherwise; Claim, Complete, and
// Fail take the run's repository ID and reach its cell directly.
//
// These endpoints are not yet part of Entire's published API and may change
// without a major SDK release.
type RunnerService service

// Error codes of RunnerService.Claim, in HTTPError.Code. None of them
// changes on a retry: skip the run.
const (
	// CodeRunNotFound: the run is unknown, not self-hosted, or one the
	// account may not see (404).
	CodeRunNotFound = "run_not_found"
	// CodeRunClaimed: another account holds the run (409).
	CodeRunClaimed = "run_claimed"
	// CodeRunNotClaimable: the run is no longer waiting: it finished, was
	// superseded by a newer commit, or passed its claim deadline (409).
	CodeRunNotClaimable = "run_not_claimable"
	// CodeRepositoryPaused: the run's repository is paused (409).
	CodeRepositoryPaused = "repository_paused"
)

// RunnerJobState is whether a job waits for a runner or the caller holds it.
type RunnerJobState string

// Runner job states.
const (
	// RunnerJobWaiting: no runner has claimed the run.
	RunnerJobWaiting RunnerJobState = "waiting"
	// RunnerJobClaimed: the caller holds the run and has not finished it.
	// Claiming it again returns it unchanged.
	RunnerJobClaimed RunnerJobState = "claimed"
)

// RunnerJob is a run a runner can claim, or already holds: enough to decide
// whether to claim it.
type RunnerJob struct {
	// RunID identifies the run.
	RunID string `json:"run_id"`
	// AgentID is the run's agent, and AgentName its name as the run pinned
	// it.
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	// ProjectID is the agent's project.
	ProjectID string `json:"project_id"`
	// RepositoryID is the repository the run reviews. Pass it to Claim,
	// Complete, and Fail.
	RepositoryID string `json:"repository_id"`
	// ChangeID is the Change the run reviews, Branch its branch, and
	// HeadSHA the commit under review.
	ChangeID string `json:"change_id"`
	Branch   string `json:"branch"`
	HeadSHA  string `json:"head_sha"`
	// Harness is the agent CLI the run is to use, and Model the model to
	// pass it; an empty Model takes the harness's default.
	Harness AgentHarness `json:"harness"`
	Model   string       `json:"model"`
	// State is RunnerJobWaiting or RunnerJobClaimed.
	State RunnerJobState `json:"state"`
	// CreatedAt is when the run was admitted.
	CreatedAt Timestamp `json:"created_at"`
	// DeadlineAt is when the run times out: while it waits, its claim
	// deadline; once claimed, the agent's timeout.
	DeadlineAt Timestamp `json:"deadline_at"`
}

// RunnerClaim is a claimed run: what to check out, what to ask, and the
// shape of the answer.
type RunnerClaim struct {
	// RunID identifies the run, and AgentID its agent.
	RunID   string `json:"run_id"`
	AgentID string `json:"agent_id"`
	// Harness is the agent CLI to run, and Model the model to pass it; an
	// empty Model takes the harness's default.
	Harness AgentHarness `json:"harness"`
	Model   string       `json:"model"`
	// ProjectID, RepositoryID, and ChangeID identify what the run
	// reviews.
	ProjectID    string `json:"project_id"`
	RepositoryID string `json:"repository_id"`
	ChangeID     string `json:"change_id"`
	// GitRemoteURL is the entire:// remote to clone the repository from.
	// git-remote-entire fetches it with the runner's service account
	// secret in ENTIRE_TOKEN.
	GitRemoteURL string `json:"git_remote_url"`
	// Branch is the Change's branch, and BaseBranch the branch it merges
	// into.
	Branch     string `json:"branch"`
	BaseBranch string `json:"base_branch"`
	// BaseSHA is the merge base the diff is taken from, and HeadSHA the
	// commit to review.
	BaseSHA string `json:"base_sha"`
	HeadSHA string `json:"head_sha"`
	// Prompt is the starter prompt: review guidance, the agent's
	// instructions, and the Change's context, including earlier open
	// findings. The runner may use, change, or replace it.
	Prompt string `json:"prompt"`
	// ResultType is the kind of result the run takes.
	ResultType string `json:"result_type"`
	// OutputSchema is the JSON Schema of the result: pass it to the
	// harness, or append it to the prompt.
	OutputSchema json.RawMessage `json:"output_schema"`
	// OutputExample is a minimal valid result, a shape to copy and not
	// something to post.
	OutputExample json.RawMessage `json:"output_example"`
	// DeadlineAt is when the run times out without a result.
	DeadlineAt Timestamp `json:"deadline_at"`
}

// RunnerCompleteRequest is the body of RunnerService.Complete.
type RunnerCompleteRequest struct {
	// Result is the review, shaped by the claim's OutputSchema.
	Result json.RawMessage `json:"result"`
	// Harness and Model are what the runner actually used, for display.
	// Entire does not verify them. Harness is required.
	Harness string `json:"harness"`
	Model   string `json:"model,omitempty"`
}

// RunnerOutcome is a run that a runner finished.
type RunnerOutcome struct {
	// RunID identifies the run.
	RunID string `json:"run_id"`
	// Status is RunSucceeded after Complete, and RunFailed after Fail.
	Status RunStatus `json:"status"`
	// ReviewID is the review Complete published the result as.
	ReviewID string `json:"review_id"`
}

// ListJobs returns one page of the runs the caller may claim, and of the
// ones it holds, oldest first. A run is listed while it waits or the caller
// holds it before its deadline, when its Environment is bound to the caller
// and its repository is one the caller is a member of and can pull. Items
// carry no prompt: Claim returns it. A paused repository's runs are listed,
// and refused by Claim.
//
// Each cell lists only the runs it processes, which for a project are in the
// cell that holds its repositories. ListJobs asks the caller's home cell,
// which is that cell when the project's jurisdiction has one. To reach the
// project's cell wherever it is, pass WithRoute(Route{RepoID: id}) with any
// repository the runner serves.
func (s *RunnerService) ListJobs(ctx context.Context, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]RunnerJob], error) {
	req := homeRequest(http.MethodGet, nil, "/self-hosted-runner/jobs")
	Deref(opts).add(req.Query, "per_page", "cursor")
	return listCursor[RunnerJob](ctx, s.client, req, reqOpts)
}

// ListJobsAll iterates over every job that ListJobs returns, page by page.
func (s *RunnerService) ListJobsAll(ctx context.Context, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[RunnerJob, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]RunnerJob], error) {
		return s.ListJobs(ctx, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// Claim leases the run runID, in repository repoID, to the caller and starts
// it: it runs until the caller completes or fails it, or its DeadlineAt
// passes. A claim by the account that holds the run returns it again
// unchanged, so a claim whose response was lost can be retried, and a
// RetryPolicy retries it. Before leasing a waiting run, Entire checks that
// its branch has not moved past HeadSHA; if it has, the run is canceled and
// the claim refused.
//
// Every claim checks again that the caller is bound to the run's
// Environment, is a member of the repository with pull on it, and that the
// repository is not paused. Refusals carry CodeRunNotFound,
// CodeRunClaimed, CodeRunNotClaimable, or CodeRepositoryPaused; a 503
// Service Unavailable claims nothing and can be retried.
func (s *RunnerService) Claim(ctx context.Context, repoID, runID string, reqOpts ...RequestOption) (*Response[RunnerClaim], error) {
	req := runnerRequest(repoID, nil, "/self-hosted-runner/jobs/%s/claim", runID)
	req.Idempotent = true
	return Call[RunnerClaim](ctx, s.client, req, reqOpts...)
}

// Complete posts the result of a run the caller claimed, which Entire
// validates as it does a hosted run's and publishes as the run's review.
//
// A result that does not parse, or whose findings point at files absent
// from the reviewed commit, fails with 422 Unprocessable Entity, whose
// message says what to fix; the run stays claimed, so correct the result and
// post it again before DeadlineAt. A 409 Conflict is final: the run
// finished, timed out, was canceled, or its Change moved past the reviewed
// commit. A 404 Not Found means the caller does not hold the run, or lost
// its binding or access. After a 503 Service Unavailable or a lost
// response, post the same result again; a RetryPolicy does that. Posting
// again never publishes a second review: once the first post has finished
// the run, a repeat fails with 409 Conflict.
func (s *RunnerService) Complete(ctx context.Context, repoID, runID string, body RunnerCompleteRequest, reqOpts ...RequestOption) (*Response[RunnerOutcome], error) {
	req := runnerRequest(repoID, body, "/self-hosted-runner/runs/%s/complete", runID)
	req.Idempotent = true
	return Call[RunnerOutcome](ctx, s.client, req, reqOpts...)
}

// Fail ends a run the caller claimed but cannot answer, so it need not wait
// out its deadline. reason, which must not be blank, becomes the run's
// error. It fails with 409 Conflict when the run has already finished, timed
// out, or been canceled, so a RetryPolicy may repeat it safely.
func (s *RunnerService) Fail(ctx context.Context, repoID, runID, reason string, reqOpts ...RequestOption) (*Response[RunnerOutcome], error) {
	body := struct {
		Error string `json:"error"`
	}{reason}
	req := runnerRequest(repoID, body, "/self-hosted-runner/runs/%s/fail", runID)
	req.Idempotent = true
	return Call[RunnerOutcome](ctx, s.client, req, reqOpts...)
}

// runnerRequest builds a POST about a run, sent to the cell of the run's
// repository.
func runnerRequest(repoID string, body any, format string, runID string) *Request {
	req := repoRequest(http.MethodPost, repoID, body, format, runID)
	if repoID == "" && req.invalid == nil {
		req.invalid = errors.New("entire: the run's repository ID must not be empty")
	}
	return req
}
