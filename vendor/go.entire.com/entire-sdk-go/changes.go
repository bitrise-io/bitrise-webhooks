package entire

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// ChangesService manages a repository's Changes. A Change is a unit of code
// work on a branch, like a pull request: its reviews and discussions, the
// gates it must pass, and its merge. Every Change belongs to a project Trail,
// which holds the work's description, type, priority, and assignees (see
// TrailsService). Changes are addressed by repository ID and number.
type ChangesService service

// ChangeStatus is a Change's lifecycle status.
type ChangeStatus string

// Change statuses.
const (
	// ChangeStatusDraft is the status of Changes opened as drafts before
	// Entire stopped creating them. New Changes are never drafts.
	ChangeStatusDraft  ChangeStatus = "draft"
	ChangeStatusOpen   ChangeStatus = "open"
	ChangeStatusMerged ChangeStatus = "merged"
	ChangeStatusClosed ChangeStatus = "closed"
)

// Change is a Change. List, Get, Create, and Update all return Change;
// fields documented as set by some of them are empty in the others.
type Change struct {
	// ID is the Change's stable ID.
	ID string `json:"id"`
	// Number is the Change's number within its repository. The
	// ChangesService methods take it as number.
	Number int64 `json:"number"`
	// URL is the Change's page on the web. It is empty when the server
	// has no web address configured.
	URL string `json:"url"`
	// Title is the Change's title, which is never empty.
	Title string `json:"title"`
	// Status is the Change's lifecycle status.
	Status ChangeStatus `json:"status"`
	// Branch is the Change's branch. It is empty when the Change has no
	// branch, such as once a merged Change's branch is gone.
	Branch string `json:"branch"`
	// OriginalBranch is the branch the Change was opened with. It stays
	// set when Branch is cleared.
	OriginalBranch string `json:"original_branch"`
	// Base is the branch the Change merges into.
	Base string `json:"base"`
	// MergedBase is the branch a merged Change landed on, without
	// "refs/heads/". A stacked Change merged through Entire names its
	// stack's base rather than its own Base. It is empty until the Change
	// merges, and for merges recorded before Entire kept it.
	MergedBase string `json:"merged_base"`
	// Author is the account that opened the Change. It is the zero
	// ChangeAuthor when the Change has no recorded author.
	Author ChangeAuthor `json:"author"`
	// Reviewers are the people asked to review the Change, with their
	// decisions.
	Reviewers []ChangeReviewer `json:"reviewers"`
	// ReadyForReview reports that the Change was marked ready for review.
	ReadyForReview bool `json:"ready_for_review"`
	// HasCodeChanges reports that the branch has commits that Base does
	// not.
	HasCodeChanges bool `json:"has_code_changes"`
	// CreatedAt is when the Change was opened.
	CreatedAt Timestamp `json:"created_at"`
	// UpdatedAt is when the Change last changed. List orders Changes by
	// it, most recent first.
	UpdatedAt Timestamp `json:"updated_at"`
	// LastActivityAt is when anything meaningful last happened on the
	// Change, leaving out background bookkeeping. It is CreatedAt until
	// then.
	LastActivityAt Timestamp `json:"last_activity_at"`
	// MergedAt is when the Change merged. It is zero until the Change
	// merges.
	MergedAt Timestamp `json:"merged_at"`
	// Parent is the project Trail that owns the Change. List and Get set
	// it when the parent is visible to the caller; nil does not prove that
	// the Change has no parent.
	Parent *ChangeParent `json:"parent"`
	// CheckpointCount counts the checkpoints on the Change's branch that
	// are not on Base. Create and Update set it.
	CheckpointCount int64 `json:"checkpoint_count"`

	// BaseSHA is the commit of Base that the Change compares against, kept
	// as it was before the Change merged. Get and Update set it; it is
	// empty when unknown.
	BaseSHA string `json:"base_sha"`
	// HeadSHA is the commit of Branch that the Change compares. Get and
	// Update set it; it is empty when unknown.
	HeadSHA string `json:"head_sha"`
	// MergeBaseSHA is the best common ancestor of BaseSHA and HeadSHA. Get
	// and Update set it; it is empty when unknown.
	MergeBaseSHA string `json:"merge_base_sha"`
	// Body is the Change's own description document. Descriptions belong
	// to the parent Trail now, so it is usually empty. Get, Create, and
	// Update set it.
	Body *BodyDocument `json:"body_document"`

	// PullRequestNumber is the number of the GitHub pull request linked to
	// the Change, or zero when there is none. Get sets it.
	PullRequestNumber int64 `json:"gh_pr_id"`
	// BranchState is the observed state of Branch, "present", "deleted",
	// "unlinked" when the Change has no branch, or "unknown", which does
	// not mean deleted. Get sets it.
	BranchState string `json:"branch_state"`
	// CodeChanges says whether the branch has code to merge: "present"
	// when it has commits ahead of Base, "none", "unknown" when there is no
	// usable evidence, or "not_applicable" for a merged, closed, or
	// unlinked Change. Get sets it.
	CodeChanges string `json:"code_changes"`
	// LifecycleSync is "pending" when the linked pull request reports a
	// merge or close, or a deleted branch, that the Change has not caught
	// up with yet, and "settled" otherwise. Get sets it.
	LifecycleSync string `json:"lifecycle_sync"`
	// Mergeability reports whether the Change can merge now. Get sets it.
	Mergeability *Mergeability `json:"mergeability"`
	// Actions says which operations the caller can perform on the Change
	// as it stands. The server checks again when one is performed. Get
	// sets it.
	Actions *ChangeActions `json:"actions"`
	// Runners are the repository's configured runners with their latest
	// run on the Change. Get sets it.
	Runners []ChangeRunner `json:"runners"`
	// Deployments are the deployments of the Change's head commit, for
	// display only: they never gate a merge. Get sets it; it is nil when
	// the server did not read them.
	Deployments *ChangeDeployments `json:"deployments"`
	// ConfigSource says where the configuration behind Runners and
	// Monitors came from: "live", "snapshot" for the configuration
	// recorded when a merged or closed Change ended, "live_fallback" for an
	// ended Change with no snapshot, or "unavailable" when it could not be
	// read, in which case empty Runners and Monitors do not mean that none
	// are configured. Get sets it.
	ConfigSource string `json:"config_source"`
	// Monitors are the repository's monitors and their latest readings on
	// the Change, as JSON. Their shape follows the web app and may change
	// without notice. Get sets it.
	Monitors json.RawMessage `json:"monitors"`
	// Agents are the saved agents that have run, or will run, on the
	// Change, with their latest run and readings, as JSON. Their shape
	// follows the web app and may change without notice; ChangeGetOptions
	// selects the history format. Get sets it.
	Agents json.RawMessage `json:"agents"`

	// TrailID is the ID of the project Trail that owns the Change.
	// Trails.GetChange sets it.
	TrailID string `json:"trailId"`
	// RepositoryID is the ID of the Change's repository. Trails.GetChange
	// sets it.
	RepositoryID string `json:"repositoryId"`
	// Repository is the repository's name, or empty when it has not been
	// resolved. Trails.GetChange sets it.
	Repository string `json:"repository"`
}

// ChangeAuthor is the account that opened a Change.
type ChangeAuthor struct {
	// AccountID is the ID of the author's Entire account.
	AccountID string `json:"id"`
	// Login is the author's forge login, such as their GitHub username. It
	// is empty when the account has none.
	Login string `json:"login"`
	// Identity describes the author for display.
	Identity Identity `json:"identity"`
}

// ChangeReviewer is a person asked to review a Change.
type ChangeReviewer struct {
	// AccountID is the ID of the reviewer's Entire account, or empty when
	// the reviewer has none.
	AccountID string `json:"actor_account_id"`
	// Login is the reviewer's forge login, such as their GitHub username.
	Login string `json:"login"`
	// Status is the reviewer's decision, such as "pending", "approved", or
	// "changes_requested". A stale approval reports "pending".
	Status string `json:"status"`
	// IsApprovalStale reports that the reviewer approved an earlier version
	// of the branch, so the approval no longer counts. Get sets it.
	IsApprovalStale bool `json:"is_approval_stale"`
	// Identity describes the reviewer for display.
	Identity Identity `json:"identity"`
}

// Identity describes a person: an Entire account, or a forge login that
// isn't linked to one. It also describes an automation, such as a plugin.
type Identity struct {
	// Kind says what the identity is: "account" for an Entire account,
	// "forgeUser" for a forge login with no account, "gitAuthor" for a Git
	// author known only by name, "agent", "automation" for a plugin or
	// another automation, or "unknown". Only the fields that apply to the
	// kind are set.
	Kind string `json:"kind"`
	// AccountID is the ID of the person's Entire account. It is set only
	// when Kind is "account".
	AccountID string `json:"account_id"`
	// DisplayName is the person's display name, or empty when there is
	// none. The API never substitutes a placeholder.
	DisplayName string `json:"display_name"`
	// AvatarURL is the URL of the person's avatar image, or empty when
	// there is none.
	AvatarURL string `json:"avatar_url"`
	// Handles are the person's logins at providers, such as on GitHub.
	Handles []Handle `json:"handles"`
	// Automation names the automation when Kind is "automation", and is
	// nil otherwise.
	Automation *AutomationIdentity `json:"automation,omitempty"`
}

// AutomationIdentity names an automation that acted on Entire.
type AutomationIdentity struct {
	// Plugin is the plugin's slug, or empty for an automation that isn't
	// a plugin.
	Plugin string `json:"plugin,omitempty"`
	// InstallationID is the plugin installation that acted. Every token
	// of an installation acts as the same author, so a plugin can edit
	// what an earlier token wrote.
	InstallationID string `json:"installation_id,omitempty"`
	// AutomationID identifies an automation that isn't a plugin.
	AutomationID string `json:"automation_id,omitempty"`
}

// Handle is a person's login at a provider.
type Handle struct {
	// Provider is the service the login belongs to, such as "github".
	Provider string `json:"provider"`
	// Handle is the login itself, such as a GitHub username.
	Handle string `json:"handle"`
	// IsPrimary marks the person's primary handle.
	IsPrimary bool `json:"is_primary"`
}

// ChangeParent is the project Trail that owns a Change.
type ChangeParent struct {
	// ID is the Trail's ID. Pass it, with ProjectPath, to
	// Trails.GetInProject.
	ID string `json:"id"`
	// Number is the Trail's number within its project, not a Change
	// number.
	Number int64 `json:"number"`
	// ProjectID is the ID of the project.
	ProjectID string `json:"project_id"`
	// Host is the first element of the project's path: "gh" or "et".
	Host string `json:"host"`
	// Project is the project's public name, the second element of its
	// path, such as "acme".
	Project string `json:"project"`
	// URL is the Trail's page on the web, or empty when the server has no
	// web address configured.
	URL string `json:"url"`
}

// ProjectPath returns the path of the Trail's project, such as "et/acme",
// which the TrailsService methods take.
func (p ChangeParent) ProjectPath() string { return p.Host + "/" + p.Project }

// BodyDocument is a rich-text description: a Trail's, or a Change's own.
type BodyDocument struct {
	// Text is the description as plain text.
	Text string `json:"text_snapshot"`
	// Content is the description as a ProseMirror document.
	Content json.RawMessage `json:"content_json"`
	// ETag identifies this version of the description. It is empty when
	// there is no description yet.
	ETag string `json:"etag"`
	// UpdatedAt is when the description last changed.
	UpdatedAt Timestamp `json:"updated_at"`
}

// ChangeActions says which operations the caller can perform on a Change.
type ChangeActions struct {
	Approve         ActionAvailability `json:"approve"`
	CreateCIPR      ActionAvailability `json:"create_ci_pr"`
	Merge           ActionAvailability `json:"merge"`
	MergeWithBypass ActionAvailability `json:"merge_with_bypass"`
	UpdateBranch    ActionAvailability `json:"update_branch"`
	RestoreBranch   ActionAvailability `json:"restore_branch"`
	DeleteBranch    ActionAvailability `json:"delete_branch"`
}

// ActionAvailability says whether the caller can perform an operation.
type ActionAvailability struct {
	// State is "enabled", "blocked" when the operation applies but
	// something prevents it now, or "unavailable".
	State string `json:"state"`
	// Reason explains a State other than "enabled", or is empty.
	Reason string `json:"reason"`
}

// ChangeRunner is a configured runner with its latest run on a Change.
type ChangeRunner struct {
	// RunnerID names the runner in the repository's configuration.
	RunnerID string `json:"runner_id"`
	// State is where the runner stands on the Change, such as "not_run",
	// "running", "evaluated", or "stale".
	State string `json:"state"`
	// Outcome is the verdict of the latest run, such as "passed" or
	// "failed", or empty when there is none.
	Outcome string `json:"outcome"`
	// RunID identifies the latest run, or is empty.
	RunID string `json:"run_id"`
	// EvaluatedAtSHA is the head commit the latest run evaluated, or is
	// empty.
	EvaluatedAtSHA string `json:"evaluated_at_sha"`
	// StartedAt is when the latest run started, or zero.
	StartedAt Timestamp `json:"started_at"`
	// StaleReason explains a "stale" State, such as "head_moved" or
	// "config_changed", or is empty.
	StaleReason string `json:"stale_reason"`
}

// DeploymentSummary counts the deployments of a Change's head commit.
type DeploymentSummary struct {
	// Availability is "available" when the deployments were read, even if
	// there are none; "unavailable" when the read failed, which does not
	// mean there are none; or "not_applicable" when the Change has no
	// head commit.
	Availability string `json:"availability"`
	// Ready counts the deployments that completed successfully.
	Ready int64 `json:"ready"`
	// Total counts the deployments that can become ready, leaving out
	// skipped and inactive ones.
	Total int64 `json:"total"`
}

// ChangeDeployments are the deployments of a Change's head commit.
type ChangeDeployments struct {
	DeploymentSummary
	// Items are the latest deployment of each resource, by plugin and key.
	Items []ChangeDeployment `json:"items"`
}

// ChangeDeployment is a deployed resource, as a plugin reported it.
type ChangeDeployment struct {
	// ID identifies the deployment.
	ID string `json:"id"`
	// Plugin is the slug of the plugin that reported it.
	Plugin string `json:"plugin"`
	// Key identifies the resource among the plugin's deployments of the
	// commit.
	Key string `json:"key"`
	// Name is the resource's display name.
	Name string `json:"name"`
	// Type is the kind of resource, or empty.
	Type string `json:"type"`
	// Group is the provider project that holds the resource, or nil.
	Group *DeploymentGroup `json:"group"`
	// Environment is the deployment's environment, such as "preview".
	Environment string `json:"environment"`
	// Status is "queued", "in_progress", "completed", or "inactive" for a
	// resource that no longer exists, whose links should not be offered.
	Status string `json:"status"`
	// Conclusion is the result of a completed deployment: "success",
	// "failure", "cancelled", or "skipped". It is empty otherwise.
	Conclusion string `json:"conclusion"`
	// EnvironmentURL is where the deployed resource can be reached, or is
	// empty.
	EnvironmentURL string `json:"environment_url"`
	// LogsURL links to the deployment's logs, or is empty.
	LogsURL string `json:"logs_url"`
	// Stale reports a queued or in-progress deployment that is past
	// DeadlineAt. Its Status is left as reported.
	Stale bool `json:"stale"`
	// ExternalUpdatedAt is when the provider last updated the deployment.
	ExternalUpdatedAt Timestamp `json:"external_updated_at"`
	// StartedAt, CompletedAt, and DeadlineAt are when the deployment
	// started, completed, and is expected to finish by, or zero.
	StartedAt   Timestamp `json:"started_at"`
	CompletedAt Timestamp `json:"completed_at"`
	DeadlineAt  Timestamp `json:"deadline_at"`
}

// DeploymentGroup is a provider project that holds deployed resources.
type DeploymentGroup struct {
	// Key identifies the project at the provider.
	Key string `json:"key"`
	// Name is the project's display name.
	Name string `json:"name"`
}

// ChangeListOptions filters ChangesService.List. Each filter matches any of
// its values, each Not filter excludes all of its values, and the filters
// combine.
type ChangeListOptions struct {
	// ID selects Changes by ID.
	ID []string `url:"id"`
	// IDNot leaves out Changes by ID.
	IDNot []string `url:"id[ne]"`
	// Number selects Changes by number.
	Number []int64 `url:"number"`
	// NumberNot leaves out Changes by number.
	NumberNot []int64 `url:"number[ne]"`
	// Status selects Changes with these lifecycle statuses.
	Status []ChangeStatus `url:"status"`
	// StatusNot leaves out Changes with these lifecycle statuses.
	StatusNot []ChangeStatus `url:"status[ne]"`
	// Author selects Changes by their authors' account IDs.
	Author []string `url:"author"`
	// AuthorNot leaves out Changes by their authors' account IDs.
	AuthorNot []string `url:"author[ne]"`
	// Query matches a substring of the title, Branch, or OriginalBranch,
	// ignoring case.
	Query string `url:"q"`
	ListOptions
}

// ChangeGetOptions configures ChangesService.Get and TrailsService.GetChange.
type ChangeGetOptions struct {
	// AgentHistory selects the history format of Change.Agents:
	// "readings", the default, lists only finished readings, and "runs"
	// also lists canceled runs, marking each entry with its kind.
	AgentHistory string `url:"agent_history"`
}

// ChangeCreateRequest is the body of ChangesService.Create. A Change carries
// only its branch; the description, type, priority, and assignees belong to
// its project Trail. Create opens a Trail for the Change; to open one inside
// an existing Trail, use TrailsService.AddChange.
type ChangeCreateRequest struct {
	// Title is the Change's title. It is required and must not be blank.
	Title string `json:"title"`
	// Base is the branch to merge into; empty selects the repository's
	// default branch.
	Base string `json:"base,omitempty"`
	// Branch is the Change's branch, which BranchAction creates or links.
	// It is required, and cannot be under "entire/".
	Branch string `json:"branch_name,omitempty"`
	// BranchAction is "create", the default, to create Branch from Base,
	// or "link" to use an existing branch.
	BranchAction string `json:"branch_action,omitempty"`
	// Status is ChangeStatusOpen or ChangeStatusClosed; empty selects
	// ChangeStatusOpen.
	Status ChangeStatus `json:"status,omitempty"`
	// ReadyForReview marks the Change ready for review. It requires Status
	// to be open.
	ReadyForReview bool `json:"ready_for_review,omitempty"`
}

// ChangeUpdateRequest is the body of ChangesService.Update. Nil fields are
// left unchanged. Merge a Change with Merge: Status cannot be set to merged.
// The description, type, priority, and assignees belong to its project
// Trail; change them with TrailsService.UpdateInProject.
type ChangeUpdateRequest struct {
	// Title replaces the Change's title, which must not be blank.
	Title *string `json:"title,omitempty"`
	// Status sets the Change's lifecycle status to ChangeStatusOpen or
	// ChangeStatusClosed. A merged Change's status cannot change.
	Status *ChangeStatus `json:"status,omitempty"`
	// Base sets the branch the Change merges into. It must not be empty.
	Base *string `json:"base,omitempty"`
	// Branch links the Change to a branch, or unlinks it with Null. The
	// branch cannot be under "entire/".
	Branch Nullable[string] `json:"branch,omitzero"`
	// ReadyForReview marks the Change ready for review, or not. Setting it
	// to true fails with 422 Unprocessable Entity unless the Change, after
	// any Status change, is open.
	ReadyForReview *bool `json:"ready_for_review,omitempty"`
}

// List returns one page of the repository's Changes, most recently updated
// first. Response.Total counts the Changes that match.
func (s *ChangesService) List(ctx context.Context, repoID string, opts *ChangeListOptions, reqOpts ...RequestOption) (*Response[[]Change], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes", repoID)
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "per_page", "cursor")
	}
	type envelope struct {
		Items      []Change `json:"items"`
		NextCursor string   `json:"next_cursor"`
		TotalCount int64    `json:"total_count"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[Change] {
		return page[Change]{items: e.Items, next: e.NextCursor, total: e.TotalCount}
	})
}

// ListAll iterates over every Change that List returns, page by page.
func (s *ChangesService) ListAll(ctx context.Context, repoID string, opts *ChangeListOptions, reqOpts ...RequestOption) iter.Seq2[Change, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Change], error) {
		p := o
		p.PageToken = token
		return s.List(ctx, repoID, &p, reqOpts...)
	})
}

// Get returns a Change with its detail: its commits, mergeability, the
// operations the caller can perform, and its runners and deployments.
func (s *ChangesService) Get(ctx context.Context, repoID string, number int64, opts *ChangeGetOptions, reqOpts ...RequestOption) (*Response[Change], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s", repoID, number)
	addQuery(req.Query, opts)
	return Call[Change](ctx, s.client, req, reqOpts...)
}

// Create opens a Change, with a new project Trail in the repository's
// project to own it. The response's ETag header is the Change's version, for
// Update with IfMatch.
func (s *ChangesService) Create(ctx context.Context, repoID string, body ChangeCreateRequest, reqOpts ...RequestOption) (*Response[Change], error) {
	type created struct {
		Change Change `json:"trail"`
	}
	req := repoRequest(http.MethodPost, repoID, body, "/repos/%s/changes", repoID)
	return unwrap(ctx, s.client, req, reqOpts, func(c *created) Change { return c.Change })
}

// Update changes a Change's fields and returns it as it now stands. With
// IfMatch and the ETag header of an earlier Create or Update response, it
// fails with 412 Precondition Failed if the Change has changed since. The
// response's ETag header is the Change's new version.
func (s *ChangesService) Update(ctx context.Context, repoID string, number int64, body ChangeUpdateRequest, reqOpts ...RequestOption) (*Response[Change], error) {
	type updated struct {
		Change Change `json:"trail"`
	}
	req := repoRequest(http.MethodPatch, repoID, body, "/repos/%s/changes/%s", repoID, number)
	return unwrap(ctx, s.client, req, reqOpts, func(u *updated) Change { return u.Change })
}

// Delete deletes a Change. A Change that belongs to a project Trail cannot
// be deleted: Delete fails with 409 Conflict.
//
// Deprecated: close the Change instead, with Update and a Status of
// ChangeStatusClosed.
func (s *ChangesService) Delete(ctx context.Context, repoID string, number int64, reqOpts ...RequestOption) (*Response[struct{}], error) {
	req := repoRequest(http.MethodDelete, repoID, nil, "/repos/%s/changes/%s", repoID, number)
	return Call[struct{}](ctx, s.client, req, reqOpts...)
}

// ChangeEvent is something that happened on a Change, such as a review
// starting, a comment changing, or a runner finishing.
type ChangeEvent struct {
	// ID identifies the event within its Change. It is a decimal number
	// that grows with each event; pass it as ChangeEventListOptions.After
	// or WatchOptions.LastEventID to resume after this event.
	ID string `json:"id"`
	// Type names what happened, such as "review.started",
	// "comment.created", or "runner.done". New types appear over time.
	Type string `json:"event_type"`
	// ReviewID is the review the event belongs to, if any.
	ReviewID string `json:"review_id"`
	// ActorID is the account that caused the event, if any.
	ActorID string `json:"actor_id"`
	// TargetType is the kind of thing the event is about, such as a
	// comment.
	TargetType string `json:"target_type"`
	// TargetID is the ID of the thing the event is about.
	TargetID string `json:"target_id"`
	// Payload carries the event's details as JSON, which depend on its
	// Type. It may be null.
	Payload json.RawMessage `json:"payload"`
	// CreatedAt is when the event happened.
	CreatedAt Timestamp `json:"created_at"`
}

// ChangeEventListOptions filters ChangesService.ListEvents.
type ChangeEventListOptions struct {
	// Types selects categories of events: "runner", "monitor", or
	// "finding".
	Types []string `url:"type"`
	// After starts the listing after this event ID instead of at the
	// beginning. It cannot be combined with a PageToken or From.
	After string `url:"after"`
	// From is "head" to start the listing at the Change's latest push
	// instead of at its first event. The first page is empty when the push
	// has no events yet, and a Change with no head lists every event. It
	// cannot be combined with a PageToken or After.
	From string `url:"from"`
	ListOptions
}

// ListEvents returns one page of a Change's recorded events, oldest first.
// WatchEvents follows new events as they happen.
func (s *ChangesService) ListEvents(ctx context.Context, repoID string, number int64, opts *ChangeEventListOptions, reqOpts ...RequestOption) (*Response[[]ChangeEvent], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/events", repoID, number)
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "per_page", "cursor")
	}
	type envelope struct {
		Events     []ChangeEvent `json:"events"`
		NextCursor string        `json:"next_cursor"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[ChangeEvent] {
		return page[ChangeEvent]{items: e.Events, next: e.NextCursor}
	})
}

// ListEventsAll iterates over every event that ListEvents returns, page by
// page.
func (s *ChangesService) ListEventsAll(ctx context.Context, repoID string, number int64, opts *ChangeEventListOptions, reqOpts ...RequestOption) iter.Seq2[ChangeEvent, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]ChangeEvent], error) {
		p := o
		if token != "" {
			p.PageToken, p.After, p.From = token, "", ""
		}
		return s.ListEvents(ctx, repoID, number, &p, reqOpts...)
	})
}

// WatchEvents iterates over a Change's events as they happen. When the
// stream ends or fails, it reconnects with the ID of the last event it
// received, so the server replays what it missed, waiting as the server
// suggests or with exponential backoff. It stops when ctx ends, when the
// caller breaks out of the loop, or when the stream cannot be opened, such as
// with 401 or 404, which it yields as the final error. Set
// WatchOptions.LastEventID to resume after an event that an earlier watch
// received.
func (s *ChangesService) WatchEvents(ctx context.Context, repoID string, number int64, options *WatchOptions, reqOpts ...RequestOption) iter.Seq2[ChangeEvent, error] {
	events := watch(ctx, options, func(lastEventID string) (*EventStream, error) {
		return s.streamEvents(ctx, repoID, number, lastEventID, reqOpts)
	})
	return func(yield func(ChangeEvent, error) bool) {
		for event, err := range events {
			if err != nil {
				yield(ChangeEvent{}, err)
				return
			}
			switch event.Type {
			case "ready", "reconnect", "error", "forbidden":
				// The stream's own frames: the connection opened, or is
				// ending, and the watch reconnects or reports why it
				// cannot.
				continue
			}
			var changeEvent ChangeEvent
			if err := event.Decode(&changeEvent); err != nil {
				yield(ChangeEvent{}, fmt.Errorf("entire: decode change event %s: %w", event.ID, err))
				return
			}
			if changeEvent.ID == "" {
				continue // not a Change event, such as a newer kind of frame
			}
			if !yield(changeEvent, nil) {
				return
			}
		}
	}
}

// streamEvents opens one Change event stream, resuming after lastEventID
// when it is set.
func (s *ChangesService) streamEvents(ctx context.Context, repoID string, number int64, lastEventID string, reqOpts []RequestOption) (*EventStream, error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/changes/%s/events", repoID, number)
	req.Header.Set("Accept", "text/event-stream")
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := s.client.Do(ctx, req, reqOpts...)
	if err != nil {
		return nil, err
	}
	return NewEventStream(resp)
}
