package entire

import (
	"context"
	"iter"
	"net/http"
)

// EnvironmentsService manages a project's Environments, which say where its
// agents run, and the runners bound to its self-hosted Environments.
// Environments are versioned and reusable across the project's agents. Its
// methods take a project path, such as "gh/acme" or "et/acme", and need
// canManageAgents on the project; binding and unbinding a runner need
// project admin.
//
// Entire keeps one managed self-hosted Environment per project, created the
// first time an agent is saved with RunsOn set to RunsOnSelfHosted. It
// cannot be replaced or archived, and an Environment cannot be created to
// run on RunsOnSelfHosted.
//
// These endpoints are not yet part of Entire's published API and may change
// without a major SDK release.
type EnvironmentsService service

// Environment is one of a project's Environments at its current version.
type Environment struct {
	// ID identifies the Environment. It never changes.
	ID string `json:"id"`
	// ProjectID is the Environment's project.
	ProjectID string `json:"project_id"`
	// Version is the current version.
	Version int64 `json:"version"`
	// Managed reports the self-hosted Environment Entire keeps for the
	// project.
	Managed bool `json:"managed"`
	// ETag is the Environment's write token. Pass it to Replace and
	// Archive.
	ETag string `json:"etag"`
	// Name and Description describe the Environment.
	Name        string `json:"name"`
	Description string `json:"description"`
	// RunsOn says whose machines run its agents. It is fixed at creation.
	RunsOn RunsOn `json:"runs_on"`
	// AutoStopMinutes is the sandbox lifetime in minutes; zero uses the
	// cell's default, as a self-hosted Environment always does.
	AutoStopMinutes int `json:"auto_stop_minutes"`
	// ClaimTimeoutMS is how long, in milliseconds, a run in a self-hosted
	// Environment waits for a runner to claim it before it times out; zero
	// waits 60 minutes.
	ClaimTimeoutMS int `json:"claim_timeout_ms"`
	// CreatedAt and CreatedBy record version 1.
	CreatedAt Timestamp   `json:"created_at"`
	CreatedBy ConfigActor `json:"created_by"`
	// UpdatedAt is when a version was last saved or the Environment
	// archived.
	UpdatedAt Timestamp `json:"updated_at"`
	// ArchivedAt is when the Environment was archived, and ArchivedBy who
	// did; zero and nil while it is active.
	ArchivedAt Timestamp    `json:"archived_at"`
	ArchivedBy *ConfigActor `json:"archived_by"`
}

// EnvironmentVersion is one saved version of an Environment.
type EnvironmentVersion struct {
	// EnvironmentID is the Environment the version belongs to.
	EnvironmentID string `json:"environment_id"`
	// Version is the version number.
	Version int64 `json:"version"`
	// The version's fields, as on Environment.
	Name            string `json:"name"`
	Description     string `json:"description"`
	RunsOn          RunsOn `json:"runs_on"`
	AutoStopMinutes int    `json:"auto_stop_minutes"`
	ClaimTimeoutMS  int    `json:"claim_timeout_ms"`
	// CreatedAt and CreatedBy record when and by whom it was saved.
	CreatedAt Timestamp   `json:"created_at"`
	CreatedBy ConfigActor `json:"created_by"`
}

// EnvironmentRequest is the body of EnvironmentsService.Create and Replace,
// and of AgentCreateRequest.Environment.
type EnvironmentRequest struct {
	// Name is the display name. It is required.
	Name string `json:"name"`
	// Description is free-form.
	Description string `json:"description,omitempty"`
	// RunsOn is RunsOnEntire, the default. A create cannot set
	// RunsOnSelfHosted, and a replace cannot change it.
	RunsOn RunsOn `json:"runs_on,omitempty"`
	// AutoStopMinutes is the sandbox lifetime, from 1 to 60 minutes; zero
	// uses the cell's default. A self-hosted Environment refuses it.
	AutoStopMinutes int `json:"auto_stop_minutes,omitempty"`
	// ClaimTimeoutMS is how long a run waits for a runner, from 60,000 to
	// 86,400,000 milliseconds; zero waits 60 minutes. Only a self-hosted
	// Environment takes it.
	ClaimTimeoutMS int `json:"claim_timeout_ms,omitempty"`
}

// EnvironmentListOptions filters EnvironmentsService.List.
type EnvironmentListOptions struct {
	// IncludeArchived also lists archived Environments.
	IncludeArchived bool `url:"include_archived"`
	ListOptions
}

// List returns one page of the project's Environments, by ID. Archived ones
// are left out unless opts.IncludeArchived is set.
func (s *EnvironmentsService) List(ctx context.Context, projectPath string, opts *EnvironmentListOptions, reqOpts ...RequestOption) (*Response[[]Environment], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/environments")
	addQuery(req.Query, opts)
	Deref(opts).ListOptions.add(req.Query, "per_page", "cursor")
	return listCursor[Environment](ctx, s.client, req, reqOpts)
}

// ListAll iterates over every Environment that List returns, page by page.
func (s *EnvironmentsService) ListAll(ctx context.Context, projectPath string, opts *EnvironmentListOptions, reqOpts ...RequestOption) iter.Seq2[Environment, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Environment], error) {
		p := o
		p.PageToken = token
		return s.List(ctx, projectPath, &p, reqOpts...)
	})
}

// Get returns an Environment at its current version, archived or not.
func (s *EnvironmentsService) Get(ctx context.Context, projectPath, environmentID string, reqOpts ...RequestOption) (*Response[Environment], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/environments/%s", environmentID)
	return Call[Environment](ctx, s.client, req, reqOpts...)
}

// Create saves a new Environment at version 1. Setting body.RunsOn to
// RunsOnSelfHosted fails with 422 Unprocessable Entity: save an agent with
// RunsOn set to RunsOnSelfHosted instead. The request carries a new
// Idempotency-Key, as AgentsService.Create does.
func (s *EnvironmentsService) Create(ctx context.Context, projectPath string, body EnvironmentRequest, reqOpts ...RequestOption) (*Response[Environment], error) {
	req := projectRequest(http.MethodPost, projectPath, body, "/environments")
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	return Call[Environment](ctx, s.client, req, reqOpts...)
}

// Replace saves a new version of an Environment. etag is the Environment's
// ETag from the last read or write: a stale one fails with 412 Precondition
// Failed. The managed Environment, and an archived one, fail with 409
// Conflict. Replacing an Environment that active agents use needs push on
// their repositories.
func (s *EnvironmentsService) Replace(ctx context.Context, projectPath, environmentID, etag string, body EnvironmentRequest, reqOpts ...RequestOption) (*Response[Environment], error) {
	req := configWrite(http.MethodPut, projectPath, etag, body, "/environments/%s", environmentID)
	return Call[Environment](ctx, s.client, req, reqOpts...)
}

// Archive archives an Environment that no active agent uses (409 Conflict
// otherwise). Its history stays readable. etag is as for Replace.
func (s *EnvironmentsService) Archive(ctx context.Context, projectPath, environmentID, etag string, reqOpts ...RequestOption) (*Response[Environment], error) {
	req := configWrite(http.MethodPost, projectPath, etag, nil, "/environments/%s/archive", environmentID)
	return Call[Environment](ctx, s.client, req, reqOpts...)
}

// ListVersions returns one page of an Environment's versions, newest first.
func (s *EnvironmentsService) ListVersions(ctx context.Context, projectPath, environmentID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]EnvironmentVersion], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/environments/%s/versions", environmentID)
	Deref(opts).add(req.Query, "per_page", "cursor")
	return listCursor[EnvironmentVersion](ctx, s.client, req, reqOpts)
}

// ListVersionsAll iterates over every version that ListVersions returns,
// page by page.
func (s *EnvironmentsService) ListVersionsAll(ctx context.Context, projectPath, environmentID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[EnvironmentVersion, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]EnvironmentVersion], error) {
		return s.ListVersions(ctx, projectPath, environmentID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// GetVersion returns one version of an Environment.
func (s *EnvironmentsService) GetVersion(ctx context.Context, projectPath, environmentID string, version int64, reqOpts ...RequestOption) (*Response[EnvironmentVersion], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/environments/%s/versions/%s", environmentID, version)
	return Call[EnvironmentVersion](ctx, s.client, req, reqOpts...)
}

// EnvironmentRunner binds a runner, a service account, to a self-hosted
// Environment.
type EnvironmentRunner struct {
	// EnvironmentID is the Environment.
	EnvironmentID string `json:"environment_id"`
	// ServiceAccountID is the runner's service account, svc_<ULID>.
	ServiceAccountID string `json:"service_account_id"`
	// BoundBy and BoundAt record the binding.
	BoundBy ConfigActor `json:"bound_by"`
	BoundAt Timestamp   `json:"bound_at"`
	// UnboundBy and UnboundAt record the unbinding; nil and zero while the
	// binding is active.
	UnboundBy *ConfigActor `json:"unbound_by"`
	UnboundAt Timestamp    `json:"unbound_at"`
}

// EnvironmentRunnerListOptions filters EnvironmentsService.ListRunners.
type EnvironmentRunnerListOptions struct {
	// IncludeUnbound also lists bindings that ended: the history.
	IncludeUnbound bool `url:"include_unbound"`
}

// BindRunner lets the service account serviceAccountID, which must belong
// to the project, claim the runs of every agent in the self-hosted
// Environment with RunnerService. For the managed Environment that is
// every self-hosted agent of the project. Binding an active binding again
// changes nothing. It needs project admin. It fails with 422 Unprocessable
// Entity when the Environment does not run on RunsOnSelfHosted or the
// account belongs to another project, and 409 Conflict when the
// Environment is archived. What the runner may read is what its grants
// allow: grant it read on the repositories its agents review.
func (s *EnvironmentsService) BindRunner(ctx context.Context, projectPath, environmentID, serviceAccountID string, reqOpts ...RequestOption) (*Response[EnvironmentRunner], error) {
	req := projectRequest(http.MethodPut, projectPath, nil, "/environments/%s/runners/%s", environmentID, serviceAccountID)
	req.Idempotent = true
	return Call[EnvironmentRunner](ctx, s.client, req, reqOpts...)
}

// UnbindRunner stops the service account claiming the Environment's runs.
// It succeeds whether or not the account was bound, and on an archived
// Environment. The binding stays in the history. It needs project admin.
func (s *EnvironmentsService) UnbindRunner(ctx context.Context, projectPath, environmentID, serviceAccountID string, reqOpts ...RequestOption) (*Response[struct{}], error) {
	req := projectRequest(http.MethodDelete, projectPath, nil, "/environments/%s/runners/%s", environmentID, serviceAccountID)
	req.Idempotent = true
	return Call[struct{}](ctx, s.client, req, reqOpts...)
}

// ListRunners returns the runners bound to an Environment, and with
// opts.IncludeUnbound the bindings that ended. It is not paginated.
func (s *EnvironmentsService) ListRunners(ctx context.Context, projectPath, environmentID string, opts *EnvironmentRunnerListOptions, reqOpts ...RequestOption) (*Response[[]EnvironmentRunner], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/environments/%s/runners", environmentID)
	addQuery(req.Query, opts)
	return listCursor[EnvironmentRunner](ctx, s.client, req, reqOpts)
}
