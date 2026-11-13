package entire

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// This file holds what lands a Change: approvals, gates, merge readiness, and
// the merge itself.

// Approval is a decision recorded on a Change: an approval, or a request for
// changes.
type Approval struct {
	// ID identifies the decision.
	ID string `json:"id"`
	// Author is the login of the person who decided.
	Author string `json:"author"`
	// Event is the decision: "approve" or "request_changes".
	Event string `json:"event"`
	// Body is the comment that came with the decision, or empty if there
	// was none. A request for changes always has one.
	Body string `json:"body"`
	// CommitSHA is the head commit that the decision applies to.
	CommitSHA string `json:"commit_sha"`
	// CreatedAt is when the decision was recorded.
	CreatedAt Timestamp `json:"created_at"`
}

// Mergeability reports whether a Change can merge now.
type Mergeability struct {
	// Mergeable reports that nothing blocks the merge: the blocking gates
	// pass and the branch does not conflict with its base. It is false for
	// a Change with no branch.
	Mergeable bool `json:"mergeable"`
	// HeadSHA is the commit the report is for, or empty if the Change has
	// no head commit.
	HeadSHA string `json:"head_sha"`
	// BehindBy counts the base branch's commits that the Change lacks. It is
	// meaningful only when ComparisonStatus is "available", and zero
	// otherwise.
	BehindBy int64 `json:"behind_by"`
	// ConflictStatus is "clean", "conflicting", or "unknown", the provider's
	// verdict for HeadSHA. A conflict blocks the merge whatever the gates
	// say. Anything short of a verdict for HeadSHA is "unknown".
	ConflictStatus string `json:"conflict_status"`
	// ComparisonStatus reports whether BehindBy was measured: "available"
	// when the up-to-date gate has compared the branch with its base, or
	// "unknown" when it has not.
	ComparisonStatus string `json:"comparison_status"`
	// BypassPolicy says who may merge despite failing gates: "nobody",
	// "admins" (people who can manage the repository's settings), or
	// "admins_author" (those, and the Change's author). It is "nobody" when
	// the server cannot establish readiness.
	BypassPolicy string `json:"bypass_policy"`
	// Gates are the gate results that the report is based on.
	Gates []Gate `json:"gates"`
	// Checks are the CI checks of HeadSHA, from the same read that
	// evaluated the checks gate.
	Checks Checks `json:"checks"`
	// ForgeMergeBlock is set when the forge's own branch rules, such as
	// GitHub branch protection, refuse the merge at HeadSHA whatever the
	// gates say. It is nil when nothing is blocked or the block is unknown.
	ForgeMergeBlock *ForgeMergeBlock `json:"forge_merge_block"`
	// DeleteBranchOnMerge is the default of MergeOptions.DeleteBranch: the
	// repository's delete-branch-on-merge setting. It is nil for a
	// GitHub-backed repository, where Entire never deletes the branch, and
	// when readiness is unavailable.
	DeleteBranchOnMerge *bool `json:"delete_branch_on_merge"`
}

// ForgeMergeBlock is why a forge refuses a merge.
type ForgeMergeBlock struct {
	// Reason is "review_required", "changes_requested", "checks_pending",
	// or "blocked" when the forge gives no readable rule. New reasons may
	// appear.
	Reason string `json:"reason"`
	// RequiredApprovingReviewCount is the number of approving reviews the
	// forge requires, or zero when it did not say.
	RequiredApprovingReviewCount int64 `json:"required_approving_review_count"`
}

// GateEvaluation is the result of a Change's configured gates.
type GateEvaluation struct {
	// Enabled reports that the repository has gates.
	Enabled bool `json:"gates_enabled"`
	// Mergeable reports that the gates allow merging: no blocking gate is
	// failed, pending, or in error. It does not consider merge conflicts
	// or how fresh the CI results are; Mergeability does.
	Mergeable bool `json:"mergeable"`
	// HeadSHA is the commit the gates were evaluated for.
	HeadSHA string `json:"head_sha"`
	// Gates are the results of each of the Change's gates.
	Gates []Gate `json:"gates"`
	// BlockingFailures are the blocking gates whose Status is "failed",
	// "pending", or "error", which keep the Change from merging.
	BlockingFailures []Gate `json:"blocking_failures"`
	// EvaluatedAt is when the most recent gate result completed, or started
	// if it is still running. It is zero when no gate has run.
	EvaluatedAt Timestamp `json:"evaluated_at"`
}

// Gate is the result of one of a Change's gates, such as a required approval,
// CI check, or review runner.
type Gate struct {
	// ID identifies the gate's result. It is empty for a gate that has not
	// run.
	ID string `json:"id"`
	// Key names the gate in the repository's configuration. A built-in
	// gate's Key is the same as its Type.
	Key string `json:"gate_key"`
	// Type is the gate's kind. The built-in kinds are "approvals",
	// "findings", "checks", and "up_to_date".
	Type string `json:"gate_type"`
	// Blocking reports a gate that must pass before the Change merges.
	Blocking bool `json:"blocking"`
	// Status is the gate's result: "passed", "failed", "pending", "error"
	// for an evaluation that could not reach a verdict, or "skipped" for a
	// gate that does not apply. A blocking gate that is failed, pending, or
	// in error blocks the merge; a skipped one does not.
	Status string `json:"status"`
	// State is where the gate's evaluation stands: "not_run", "running",
	// "evaluated", "stale", "disabled", "not_applicable", or
	// "configuration_error". It is set on the gates of a Mergeability, and
	// empty on those that ChangesService.Gates returns.
	State string `json:"state"`
	// Outcome is the verdict of an evaluated or stale gate: "passed",
	// "failed", or "errored". It is empty otherwise, and wherever State is.
	Outcome string `json:"outcome"`
	// Rationale explains the result, or is empty.
	Rationale string `json:"rationale"`
	// FindingCount counts the findings behind the result. It is nil for a
	// gate that does not count findings, and zero for one that counted
	// none.
	FindingCount *int64 `json:"finding_count"`
	// HeadSHA is the commit the gate was evaluated for.
	HeadSHA string `json:"head_sha"`
	// Value is the gate's measured value, whose shape depends on its Type.
	// It is nil when the gate has none.
	Value json.RawMessage `json:"value"`
	// Reviewers are the reviews the gate counts at HeadSHA, for approval
	// gates.
	Reviewers []GateReviewer `json:"reviewers"`
	// CreatedAt is when the gate began evaluating.
	CreatedAt Timestamp `json:"created_at"`
	// CompletedAt is when the evaluation finished, or zero while it is
	// still running.
	CompletedAt Timestamp `json:"completed_at"`
}

// GateReviewer is a reviewer that an approval gate counts.
type GateReviewer struct {
	// Login is the reviewer's login.
	Login string `json:"login"`
	// State is "pending", "approved", "changes_requested", "stale" for a
	// review of an earlier head commit, or "excluded" for an approval that
	// does not count, such as the author's own.
	State string `json:"state"`
	// ReviewedHeadSHA is the commit the reviewer reviewed, or empty if they
	// haven't.
	ReviewedHeadSHA string `json:"reviewed_head_sha"`
	// Reason explains a stale or excluded State: "head_changed" or
	// "self_approval_not_allowed". It is empty otherwise.
	Reason string `json:"reason"`
}

// Checks are the CI checks of a Change's head commit.
type Checks struct {
	// Availability is "available" when the runs are complete, which includes
	// no runs; "unavailable" when the evidence is missing; or
	// "not_applicable" when no CI applies. Runs is meaningful only when it
	// is "available".
	Availability string `json:"availability"`
	// Runs are the third-party CI runs of the commit. They leave out
	// Entire's own gate check.
	Runs []CheckRun `json:"runs"`
}

// CheckRun is one CI check.
type CheckRun struct {
	// Name is the check's name in the CI system.
	Name string `json:"name"`
	// Status is the run's progress, such as "queued", "in_progress", or
	// "completed".
	Status string `json:"status"`
	// Conclusion is the run's result once Status is "completed", such as
	// "success" or "failure", and empty before then.
	Conclusion string `json:"conclusion"`
	// AppName is the CI system that ran the check, if known.
	AppName string `json:"app_name"`
	// DetailsURL links to the run in its CI system, if known.
	DetailsURL string `json:"details_url"`
	// StartedAt is when the run started, or zero if it hasn't.
	StartedAt Timestamp `json:"started_at"`
	// CompletedAt is when the run completed, or zero if it hasn't.
	CompletedAt Timestamp `json:"completed_at"`
	// Source is the slug of the CI plugin that reported the run, and empty
	// for a run read from the forge. CommitChecks.SourceAvailability is keyed
	// by it.
	Source string `json:"source"`
	// Steps are the run's steps, when its CI plugin reports them.
	Steps []CheckRunStep `json:"steps"`
}

// CheckRunStep is one step of a CI check run.
type CheckRunStep struct {
	// ID identifies the step within its run.
	ID string `json:"id"`
	// Name is the step's name in the CI system.
	Name string `json:"name"`
	// Status is the step's progress, such as "queued", "in_progress", or
	// "completed".
	Status string `json:"status"`
	// Conclusion is the step's result once Status is "completed", and empty
	// before then.
	Conclusion string `json:"conclusion"`
	// ExitStatus is the step's exit code, when its CI plugin reports one.
	ExitStatus *int `json:"exit_status"`
}

// MergeOptions configures ChangesService.Merge.
type MergeOptions struct {
	// ExpectedHeadSHA merges only if the Change's head is still this commit;
	// otherwise Merge fails with 409 Conflict. Empty merges whatever the
	// head is.
	ExpectedHeadSHA string `json:"expectedHeadSha,omitempty"`
	// Bypass merges despite failing gates, when the repository's bypass
	// policy allows the caller to. Merge fails with 403 Forbidden when it
	// does not.
	Bypass bool `json:"bypass,omitempty"`
	// DeleteBranch deletes an Entire-hosted Change's branch after the merge,
	// first moving the open Changes based on it to the merged Change's base.
	// Nil follows the repository's setting. GitHub-backed Changes ignore it.
	DeleteBranch *bool `json:"deleteBranch,omitempty"`
}

// MergeResult is the outcome of a merge.
type MergeResult struct {
	// CommitSHA is the merge commit. It is empty when the merge
	// fast-forwarded or the base already had the Change's commits.
	CommitSHA string `json:"mergeCommitSha"`
}

// ListApprovals returns the approval decisions recorded on a Change.
func (s *ChangesService) ListApprovals(ctx context.Context, repoID string, number int64, reqOpts ...RequestOption) (*Response[[]Approval], error) {
	type envelope struct {
		Approvals []Approval `json:"approvals"`
	}
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/approvals", repoID, number)
	return unwrap(ctx, s.client, req, reqOpts, func(e *envelope) []Approval { return e.Approvals })
}

// Approve approves a Change's current head commit, with an optional comment.
func (s *ChangesService) Approve(ctx context.Context, repoID string, number int64, comment string, reqOpts ...RequestOption) (*Response[Approval], error) {
	return s.decide(ctx, repoID, number, "approve", comment, reqOpts)
}

// RequestChanges asks for changes to a Change, saying which in comment, which
// must not be empty.
func (s *ChangesService) RequestChanges(ctx context.Context, repoID string, number int64, comment string, reqOpts ...RequestOption) (*Response[Approval], error) {
	return s.decide(ctx, repoID, number, "request_changes", comment, reqOpts)
}

func (s *ChangesService) decide(ctx context.Context, repoID string, number int64, event, comment string, reqOpts []RequestOption) (*Response[Approval], error) {
	body := struct {
		Event string `json:"event"`
		Body  string `json:"body,omitempty"`
	}{event, comment}
	req := repoRequest(http.MethodPost, repoID, body, "/repos/%s/changes/%s/approvals", repoID, number)
	if event == "request_changes" && strings.TrimSpace(comment) == "" {
		req.invalid = errors.New("entire: Changes.RequestChanges needs a comment")
	}
	type envelope struct {
		Approval Approval `json:"approval"`
	}
	return unwrap(ctx, s.client, req, reqOpts, func(e *envelope) Approval { return e.Approval })
}

// Mergeability reports whether a Change can merge now: its gates, CI checks,
// conflicts, and how far it is behind its base.
func (s *ChangesService) Mergeability(ctx context.Context, repoID string, number int64, reqOpts ...RequestOption) (*Response[Mergeability], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/mergeability", repoID, number)
	return Call[Mergeability](ctx, s.client, req, reqOpts...)
}

// Gates evaluates a Change's configured gates.
func (s *ChangesService) Gates(ctx context.Context, repoID string, number int64, reqOpts ...RequestOption) (*Response[GateEvaluation], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/gates", repoID, number)
	return Call[GateEvaluation](ctx, s.client, req, reqOpts...)
}

// Merge merges a Change into its base branch. It fails with:
//   - 409 Conflict when another merge is in progress, when
//     MergeOptions.ExpectedHeadSHA no longer names the branch's head (the
//     detail starts with "merge_head_mismatch"), or when the branch
//     conflicts with its base;
//   - 412 Precondition Failed when the base or the branch moved during the
//     merge;
//   - 422 Unprocessable Entity when gates are pending or blocking, or the
//     Change is not open or has no branch;
//   - 403 Forbidden when the caller may not merge, or may not bypass gates;
//   - 401 Unauthorized when the forge credential behind a GitHub-backed
//     Change needs reconnecting, though the caller's own token is fine.
//
// After a 409 or 412, read the Change again before retrying. Merge is not
// repeated automatically after a 401, because the merge may have started.
func (s *ChangesService) Merge(ctx context.Context, repoID string, number int64, opts *MergeOptions, reqOpts ...RequestOption) (*Response[MergeResult], error) {
	if opts == nil {
		opts = &MergeOptions{}
	}
	req := repoRequest(http.MethodPost, repoID, opts, "/repos/%s/changes/%s/merge", repoID, number)
	return Call[MergeResult](ctx, s.client, req, reqOpts...)
}
