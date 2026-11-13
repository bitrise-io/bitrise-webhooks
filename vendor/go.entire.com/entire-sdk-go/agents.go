package entire

import (
	"context"
	"errors"
	"iter"
	"net/http"
)

// AgentsService manages a project's agents: what each runs, the
// repositories it reviews, and when. It also reads a repository's agents and
// Entire's catalog of harnesses, models, prices, and presets.
//
// An agent reviews Changes. Saving one runs nothing; its push trigger, or
// RunsService.Start, runs it in its Environment. An agent whose Environment
// runs on RunsOnSelfHosted is reviewed by a runner the project operates
// (see RunnerService).
//
// The project methods take a project path, such as "gh/acme" or "et/acme",
// and need canManageAgents on the project. A write that changes what runs in
// a repository also needs push on it.
//
// These endpoints are not yet part of Entire's published API and may change
// without a major SDK release.
type AgentsService service

// AgentHarness is the agent CLI that runs an agent's instructions.
type AgentHarness string

// Agent harnesses.
const (
	AgentHarnessClaude AgentHarness = "claude"
	AgentHarnessCodex  AgentHarness = "codex"
	AgentHarnessPi     AgentHarness = "pi"
)

// ReasoningEffort is how hard an agent's model reasons. The models an
// AgentsService.Models entry lists each accept some of them.
type ReasoningEffort string

// Reasoning efforts, lowest first.
const (
	ReasoningEffortLow    ReasoningEffort = "low"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortHigh   ReasoningEffort = "high"
	ReasoningEffortXHigh  ReasoningEffort = "xhigh"
	ReasoningEffortMax    ReasoningEffort = "max"
	ReasoningEffortUltra  ReasoningEffort = "ultra"
)

// RunsOn says whose machines run an Environment's agents. More values will
// be added.
type RunsOn string

// Where agents run.
const (
	// RunsOnEntire runs each run in a sandbox Entire provides.
	RunsOnEntire RunsOn = "entire"
	// RunsOnSelfHosted parks each run until a runner the project operates
	// claims it with RunnerService.Claim.
	RunsOnSelfHosted RunsOn = "self_hosted"
)

// AgentTriggerKind is what starts an agent.
type AgentTriggerKind string

// Agent trigger kinds.
const (
	// AgentTriggerPush runs the agent on a push to a branch with an open
	// Change in a selected repository, as its When admits.
	AgentTriggerPush AgentTriggerKind = "push"
)

// AgentTriggerWhen selects which Changes a push trigger runs on, by their
// readiness for review.
type AgentTriggerWhen string

// Push trigger conditions.
const (
	// AgentTriggerWhenReady runs only on a Change ready for review,
	// including when one becomes ready. It is the default.
	AgentTriggerWhenReady AgentTriggerWhen = "ready"
	// AgentTriggerWhenAlways runs on every push, ready or not.
	AgentTriggerWhenAlways AgentTriggerWhen = "always"
	// AgentTriggerWhenNotReady runs only on a Change not yet ready for
	// review.
	AgentTriggerWhenNotReady AgentTriggerWhen = "not_ready"
)

// SignalValueType is the kind of reading an agent's signal reports.
type SignalValueType string

// Signal value types.
const (
	SignalBoolean SignalValueType = "boolean"
	SignalPercent SignalValueType = "percent"
	SignalSize    SignalValueType = "size"
)

// SignalPolarity says which signal readings are good.
type SignalPolarity string

// Signal polarities.
const (
	SignalHigherIsBetter SignalPolarity = "higher_is_better"
	SignalLowerIsBetter  SignalPolarity = "lower_is_better"
	SignalNeutral        SignalPolarity = "neutral"
)

// AgentSignal is the one reading each review distills to.
type AgentSignal struct {
	// ValueType is SignalBoolean, SignalPercent (0 to 100), or SignalSize
	// (small, medium, or large).
	ValueType SignalValueType `json:"value_type"`
	// Polarity says which readings are good. Reads always set it; a write
	// that leaves it empty saves SignalNeutral.
	Polarity SignalPolarity `json:"polarity,omitempty"`
}

// AgentTrigger starts an agent. An agent has at most one.
type AgentTrigger struct {
	// ID is the trigger's stable ID within the agent: lowercase letters,
	// digits, - and _. A write that leaves it empty uses Kind. Keep it to
	// keep the trigger's push history.
	ID string `json:"id,omitempty"`
	// Kind is AgentTriggerPush.
	Kind AgentTriggerKind `json:"kind"`
	// When selects Changes by readiness for review; empty on a write means
	// AgentTriggerWhenReady.
	When AgentTriggerWhen `json:"when,omitempty"`
	// Branches are branch name globs, such as "main" or "release/*"; the
	// trigger runs only on a Change whose branch one matches. Empty matches
	// every branch.
	Branches []string `json:"branches,omitempty"`
	// Paths are repository path globs, such as "frontend/**"; the trigger
	// runs only on a Change that changes a path one matches. Empty matches
	// every Change. Manual runs ignore Branches and Paths.
	Paths []string `json:"paths,omitempty"`
}

// AgentPresetRef names the Entire preset that an agent runs.
type AgentPresetRef struct {
	// ID is the preset's ID, as AgentsService.ListPresets lists it.
	ID string `json:"id"`
	// Version is the preset's current version: what the agent's next run
	// runs.
	Version int `json:"version"`
}

// AgentDefinition is what an agent is called and runs. A preset agent's is
// its preset's as the catalog has it now.
type AgentDefinition struct {
	// Name is the display name, unique among the project's active agents,
	// ignoring case.
	Name string `json:"name"`
	// Description is free-form; empty when unset.
	Description string `json:"description"`
	// Harness is the agent CLI that runs the instructions.
	Harness AgentHarness `json:"harness"`
	// Model is the model passed to the harness; empty uses the harness's
	// default.
	Model string `json:"model"`
	// ReasoningEffort is the effort the agent runs at; empty when its model
	// takes none.
	ReasoningEffort ReasoningEffort `json:"reasoning_effort"`
	// TimeoutMS bounds the agent's process, in milliseconds. A self-hosted
	// run's deadline counts from its claim.
	TimeoutMS int `json:"timeout_ms"`
	// Instructions are the review focus.
	Instructions string `json:"instructions"`
	// Signal is the reading each review reports.
	Signal AgentSignal `json:"signal"`
}

// ConfigActor is who saved, archived, or paused an agent or Environment, or
// started a run.
type ConfigActor struct {
	// Kind is "account", "service_account", or "automation". More kinds may
	// be added.
	Kind string `json:"kind"`
	// ID is an account ID, svc_<ULID> for a service account, or
	// aut_<ULID> for an automation.
	ID string `json:"id"`
}

// Agent is one of a project's agents at its current version.
type Agent struct {
	// ID identifies the agent. It never changes, including through a
	// restore or a customization of a preset agent.
	ID string `json:"id"`
	// ProjectID is the agent's project.
	ProjectID string `json:"project_id"`
	// Version is the current version. A version is saved when what the
	// agent runs changes.
	Version int64 `json:"version"`
	// ETag is the agent's write token. Pass it to Replace, Archive, Pause,
	// Resume, and Restore; it changes on every write.
	ETag string `json:"etag"`
	AgentDefinition
	// Preset is the Entire preset the agent runs, or nil for a custom
	// agent.
	Preset *AgentPresetRef `json:"preset"`
	// EnvironmentID is the Environment the agent runs in.
	EnvironmentID string `json:"environment_id"`
	// RunsOn is the Environment's RunsOn.
	RunsOn RunsOn `json:"runs_on"`
	// Triggers holds the agent's trigger, if any. Without one, the agent
	// runs only when started with RunsService.Start.
	Triggers []AgentTrigger `json:"triggers"`
	// IsEnabled reports whether the trigger runs the agent; it is false
	// while the agent is paused.
	IsEnabled bool `json:"is_enabled"`
	// RepositoryIDs are the repositories the agent reviews. List leaves it
	// empty; Get, Create, and the other single-agent methods set it.
	RepositoryIDs []string `json:"repository_ids"`
	// RepositoryCount is how many repositories the agent selects.
	RepositoryCount int `json:"repository_count"`
	// StaleRepositoryIDs are selected repositories the project no longer
	// holds here. They grant nothing; a Replace can drop them.
	StaleRepositoryIDs []string `json:"stale_repository_ids"`
	// CreatedAt and CreatedBy record version 1.
	CreatedAt Timestamp   `json:"created_at"`
	CreatedBy ConfigActor `json:"created_by"`
	// UpdatedAt is when the agent was last written.
	UpdatedAt Timestamp `json:"updated_at"`
	// ArchivedAt is when the agent was archived, and ArchivedBy who did;
	// zero and nil while it is active.
	ArchivedAt Timestamp    `json:"archived_at"`
	ArchivedBy *ConfigActor `json:"archived_by"`
	// PausedAt is when the agent was paused, and PausedBy who did; zero
	// and nil while it is enabled.
	PausedAt Timestamp    `json:"paused_at"`
	PausedBy *ConfigActor `json:"paused_by"`
}

// AgentVersion is one saved version of an agent.
type AgentVersion struct {
	// AgentID is the agent the version belongs to.
	AgentID string `json:"agent_id"`
	// Version is the version number.
	Version int64 `json:"version"`
	AgentDefinition
	// Preset is the preset the version ran, or nil for a custom agent.
	Preset *AgentPresetRef `json:"preset"`
	// EnvironmentID is the Environment the version runs in.
	EnvironmentID string `json:"environment_id"`
	// Triggers holds the version's trigger, if any.
	Triggers []AgentTrigger `json:"triggers"`
	// CreatedAt and CreatedBy record when and by whom it was saved.
	CreatedAt Timestamp   `json:"created_at"`
	CreatedBy ConfigActor `json:"created_by"`
}

// AgentEventKind is what an AgentEvent records.
type AgentEventKind string

// Agent event kinds.
const (
	AgentEventCreated           AgentEventKind = "created"
	AgentEventEdited            AgentEventKind = "edited"
	AgentEventRepositoryAdded   AgentEventKind = "repository_added"
	AgentEventRepositoryRemoved AgentEventKind = "repository_removed"
	AgentEventPaused            AgentEventKind = "paused"
	AgentEventResumed           AgentEventKind = "resumed"
	AgentEventArchived          AgentEventKind = "archived"
	AgentEventRestored          AgentEventKind = "restored"
)

// AgentEvent is one change in an agent's history.
type AgentEvent struct {
	// ID identifies the event.
	ID string `json:"id"`
	// Kind is what changed.
	Kind AgentEventKind `json:"kind"`
	// RepositoryID is the repository added or removed; empty for other
	// kinds.
	RepositoryID string `json:"repository_id"`
	// Version is the version that a created or edited event saved; zero
	// for other kinds.
	Version int64 `json:"version"`
	// Changed lists, for an edited event, the fields the version changed,
	// such as "model" or "instructions".
	Changed []string `json:"changed"`
	// Actor is who made the change.
	Actor ConfigActor `json:"actor"`
	// CreatedAt is when.
	CreatedAt Timestamp `json:"created_at"`
}

// AgentCreateRequest is the body of AgentsService.Create. A preset agent
// sends Preset and leaves the definition fields empty; a custom agent sends
// Name, Harness, TimeoutMS, Instructions, and Signal. Name the Environment
// with exactly one of EnvironmentID, Environment, or RunsOn set to
// RunsOnSelfHosted alone.
type AgentCreateRequest struct {
	// Preset runs an Entire preset, as AgentsService.ListPresets lists it.
	// The agent then follows the preset as it changes.
	Preset string `json:"preset,omitempty"`
	// Name is the display name. It is required without a preset.
	Name string `json:"name,omitempty"`
	// Description is free-form.
	Description string `json:"description,omitempty"`
	// IsEnabled starts the agent enabled. False, the zero value, creates it
	// paused, so it runs only when started manually until Resume. It is
	// always sent, so the request says which one it means.
	IsEnabled bool `json:"is_enabled"`
	// Harness is the agent CLI. It is required without a preset.
	Harness AgentHarness `json:"harness,omitempty"`
	// Model is a model AgentsService.Models lists for the harness; empty
	// uses the harness's default.
	Model string `json:"model,omitempty"`
	// ReasoningEffort is one the model accepts; empty uses the harness's
	// default.
	ReasoningEffort ReasoningEffort `json:"reasoning_effort,omitempty"`
	// TimeoutMS bounds the agent's process, from 1 to 1,800,000
	// milliseconds. It is required without a preset.
	TimeoutMS int `json:"timeout_ms,omitempty"`
	// Instructions are the review focus, with {{branch}} and
	// {{base_branch}} placeholders. They are required without a preset.
	Instructions string `json:"instructions,omitempty"`
	// Signal is the reading each review reports. It is required without a
	// preset.
	Signal *AgentSignal `json:"signal,omitempty"`
	// RepositoryIDs are the repositories of the project to review. A
	// create needs at least one, and push on each.
	RepositoryIDs []string `json:"repository_ids"`
	// Triggers holds at most one trigger. Leave it empty for an agent that
	// runs only when started.
	Triggers []AgentTrigger `json:"triggers,omitempty"`
	// EnvironmentID names an active Environment of the project.
	EnvironmentID string `json:"environment_id,omitempty"`
	// Environment saves a new Environment with the agent, as
	// EnvironmentsService.Create would. It cannot run on RunsOnSelfHosted.
	Environment *EnvironmentRequest `json:"environment,omitempty"`
	// RunsOn set to RunsOnSelfHosted alone runs the agent in the project's
	// managed self-hosted Environment, created on first use. It needs push
	// on every selected repository, and a preset agent cannot use it.
	// Beside EnvironmentID or Environment it must equal that Environment's
	// RunsOn.
	RunsOn RunsOn `json:"runs_on,omitempty"`
}

// AgentReplaceRequest is the body of AgentsService.Replace: the whole agent,
// as for a create, with the Environment named by EnvironmentID or by RunsOn
// set to RunsOnSelfHosted. Replacing a preset agent with an empty Preset and
// every definition field customizes it under the same ID.
type AgentReplaceRequest struct {
	// Preset keeps or selects an Entire preset; empty makes a custom
	// agent.
	Preset string `json:"preset,omitempty"`
	// Name is the display name. It is required without a preset.
	Name string `json:"name,omitempty"`
	// Description is free-form.
	Description string `json:"description,omitempty"`
	// IsEnabled, when set, must equal the agent's state. Use Pause and
	// Resume to change it.
	IsEnabled *bool `json:"is_enabled,omitempty"`
	// Harness is the agent CLI. It is required without a preset.
	Harness AgentHarness `json:"harness,omitempty"`
	// Model is a model AgentsService.Models lists for the harness; empty
	// uses the harness's default.
	Model string `json:"model,omitempty"`
	// ReasoningEffort is one the model accepts; empty uses the harness's
	// default.
	ReasoningEffort ReasoningEffort `json:"reasoning_effort,omitempty"`
	// TimeoutMS bounds the agent's process, in milliseconds. It is
	// required without a preset.
	TimeoutMS int `json:"timeout_ms,omitempty"`
	// Instructions are the review focus. They are required without a
	// preset.
	Instructions string `json:"instructions,omitempty"`
	// Signal is the reading each review reports. It is required without a
	// preset.
	Signal *AgentSignal `json:"signal,omitempty"`
	// RepositoryIDs is the whole selection, and is required: a nil slice
	// is refused, and an empty one selects nothing. Adding a repository
	// needs push on it.
	RepositoryIDs []string `json:"repository_ids"`
	// Triggers holds at most one trigger.
	Triggers []AgentTrigger `json:"triggers,omitempty"`
	// EnvironmentID names an active Environment of the project.
	EnvironmentID string `json:"environment_id,omitempty"`
	// RunsOn set to RunsOnSelfHosted alone moves the agent to the
	// project's managed self-hosted Environment. Beside EnvironmentID it
	// must equal that Environment's RunsOn.
	RunsOn RunsOn `json:"runs_on,omitempty"`
}

// AgentListOptions filters AgentsService.List.
type AgentListOptions struct {
	// IncludeArchived also lists archived agents.
	IncludeArchived bool `url:"include_archived"`
	// RepositoryID lists only the agents that select this repository.
	RepositoryID string `url:"repository_id"`
	ListOptions
}

// cursorPage is the list envelope of the agent, Environment, run, and runner
// routes.
type cursorPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"`
}

// listCursor sends req and returns the items of a cursorPage.
func listCursor[T any](ctx context.Context, c *Client, req *Request, opts []RequestOption) (*Response[[]T], error) {
	return list(ctx, c, req, opts, func(e *cursorPage[T]) page[T] {
		return page[T]{items: e.Items, next: e.NextCursor}
	})
}

// projectRequest builds a request for a route under a project path, such as
// "gh/acme", which reaches the project's cell. The path and the route both
// use the project name in lower case, which Core matches regardless of case,
// so "gh/Acme" and "gh/acme" share one cached route.
func projectRequest(method, projectPath string, body any, suffix string, args ...any) *Request {
	host, name, err := projectTrailPath(projectPath)
	route := Route{ProjectPath: host + "/" + name}
	if err != nil {
		route = Route{ProjectPath: projectPath}
	}
	req := newRequest(method, route, false, body, "/%s/%s"+suffix, append([]any{host, name}, args...))
	if err != nil {
		req.invalid = err
	}
	return req
}

// configWrite builds a write to an existing agent or Environment, which the
// API makes conditional on its write token.
func configWrite(method, projectPath, etag string, body any, suffix string, args ...any) *Request {
	req := projectRequest(method, projectPath, body, suffix, args...)
	req.Header.Set("If-Match", etag)
	if etag == "" && req.invalid == nil {
		req.invalid = errors.New("entire: etag must not be empty; pass the ETag of the last read or write")
	}
	return req
}

// List returns one page of the project's agents, by ID. Archived agents are
// left out unless opts.IncludeArchived is set. The agents' RepositoryIDs are
// empty; Get returns them.
func (s *AgentsService) List(ctx context.Context, projectPath string, opts *AgentListOptions, reqOpts ...RequestOption) (*Response[[]Agent], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/agents")
	addQuery(req.Query, opts)
	Deref(opts).ListOptions.add(req.Query, "per_page", "cursor")
	return listCursor[Agent](ctx, s.client, req, reqOpts)
}

// ListAll iterates over every agent that List returns, page by page.
func (s *AgentsService) ListAll(ctx context.Context, projectPath string, opts *AgentListOptions, reqOpts ...RequestOption) iter.Seq2[Agent, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Agent], error) {
		p := o
		p.PageToken = token
		return s.List(ctx, projectPath, &p, reqOpts...)
	})
}

// Get returns an agent at its current version, archived or not.
func (s *AgentsService) Get(ctx context.Context, projectPath, agentID string, reqOpts ...RequestOption) (*Response[Agent], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/agents/%s", agentID)
	return Call[Agent](ctx, s.client, req, reqOpts...)
}

// Create saves a new agent at version 1. It runs nothing. It needs push on
// every repository the agent selects, and fails with 409 Conflict when an
// active agent already has the name or runs the preset. The request carries
// a new Idempotency-Key, so a RetryPolicy may resend it without saving a
// second agent; pass WithHeader("Idempotency-Key", key) to retry with your
// own key across calls.
func (s *AgentsService) Create(ctx context.Context, projectPath string, body AgentCreateRequest, reqOpts ...RequestOption) (*Response[Agent], error) {
	req := projectRequest(http.MethodPost, projectPath, body, "/agents")
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	return Call[Agent](ctx, s.client, req, reqOpts...)
}

// Replace replaces an agent with body, saving a new version when what it
// runs changes. etag is the agent's ETag from the last read or write: a
// stale one fails with 412 Precondition Failed, and an archived agent with
// 409 Conflict. It needs push on each repository it adds, and on each it
// keeps when it saves a version.
func (s *AgentsService) Replace(ctx context.Context, projectPath, agentID, etag string, body AgentReplaceRequest, reqOpts ...RequestOption) (*Response[Agent], error) {
	req := configWrite(http.MethodPut, projectPath, etag, body, "/agents/%s", agentID)
	return Call[Agent](ctx, s.client, req, reqOpts...)
}

// Archive archives an agent, which stops its trigger and cancels its queued
// and running runs. Restore undoes it. etag is as for Replace.
func (s *AgentsService) Archive(ctx context.Context, projectPath, agentID, etag string, reqOpts ...RequestOption) (*Response[Agent], error) {
	return s.lifecycle(ctx, projectPath, agentID, etag, "archive", reqOpts)
}

// Restore makes an archived agent active again, paused or not as it was.
// Its name and preset must still be free (409 Conflict otherwise), and its
// Environment active. etag is as for Replace.
func (s *AgentsService) Restore(ctx context.Context, projectPath, agentID, etag string, reqOpts ...RequestOption) (*Response[Agent], error) {
	return s.lifecycle(ctx, projectPath, agentID, etag, "restore", reqOpts)
}

// Pause stops an agent's trigger and cancels its queued and running runs.
// RunsService.Start still runs it for a caller with canManageAgents. Pausing
// a paused agent changes nothing. etag is as for Replace.
func (s *AgentsService) Pause(ctx context.Context, projectPath, agentID, etag string, reqOpts ...RequestOption) (*Response[Agent], error) {
	return s.lifecycle(ctx, projectPath, agentID, etag, "pause", reqOpts)
}

// Resume lets a paused agent's trigger run it again. It fails with 422
// Unprocessable Entity when the agent's saved version can no longer run.
// etag is as for Replace.
func (s *AgentsService) Resume(ctx context.Context, projectPath, agentID, etag string, reqOpts ...RequestOption) (*Response[Agent], error) {
	return s.lifecycle(ctx, projectPath, agentID, etag, "resume", reqOpts)
}

func (s *AgentsService) lifecycle(ctx context.Context, projectPath, agentID, etag, action string, reqOpts []RequestOption) (*Response[Agent], error) {
	req := configWrite(http.MethodPost, projectPath, etag, nil, "/agents/%s/"+action, agentID)
	return Call[Agent](ctx, s.client, req, reqOpts...)
}

// ListVersions returns one page of an agent's versions, newest first.
func (s *AgentsService) ListVersions(ctx context.Context, projectPath, agentID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]AgentVersion], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/agents/%s/versions", agentID)
	Deref(opts).add(req.Query, "per_page", "cursor")
	return listCursor[AgentVersion](ctx, s.client, req, reqOpts)
}

// ListVersionsAll iterates over every version that ListVersions returns,
// page by page.
func (s *AgentsService) ListVersionsAll(ctx context.Context, projectPath, agentID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[AgentVersion, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]AgentVersion], error) {
		return s.ListVersions(ctx, projectPath, agentID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// GetVersion returns one version of an agent.
func (s *AgentsService) GetVersion(ctx context.Context, projectPath, agentID string, version int64, reqOpts ...RequestOption) (*Response[AgentVersion], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/agents/%s/versions/%s", agentID, version)
	return Call[AgentVersion](ctx, s.client, req, reqOpts...)
}

// ListEvents returns one page of an agent's history, newest first: each
// saved version, selection change, and pause, resume, archive, or restore.
func (s *AgentsService) ListEvents(ctx context.Context, projectPath, agentID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]AgentEvent], error) {
	req := projectRequest(http.MethodGet, projectPath, nil, "/agents/%s/events", agentID)
	Deref(opts).add(req.Query, "per_page", "cursor")
	return listCursor[AgentEvent](ctx, s.client, req, reqOpts)
}

// ListEventsAll iterates over every event that ListEvents returns, page by
// page.
func (s *AgentsService) ListEventsAll(ctx context.Context, projectPath, agentID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[AgentEvent, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]AgentEvent], error) {
		return s.ListEvents(ctx, projectPath, agentID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// RepositoryAgent summarizes an agent for a reader of one repository it
// reviews: nothing about the other repositories it selects, its
// Environment, or its instructions.
type RepositoryAgent struct {
	// ID identifies the agent.
	ID string `json:"id"`
	// Name and Description are the agent's; a preset agent's are its
	// preset's.
	Name        string `json:"name"`
	Description string `json:"description"`
	// Preset is the Entire preset the agent runs, or nil for a custom
	// agent.
	Preset *AgentPresetRef `json:"preset"`
	// Harness and Model are what runs; an empty Model is the harness's
	// default.
	Harness AgentHarness `json:"harness"`
	Model   string       `json:"model"`
	// Signal is the reading each review reports.
	Signal AgentSignal `json:"signal"`
	// IsEnabled reports whether the trigger runs the agent.
	IsEnabled bool `json:"is_enabled"`
	// Triggers holds the agent's trigger, if any.
	Triggers []AgentTrigger `json:"triggers"`
	// LatestRun is the agent's latest run in the repository, finished or
	// not, or nil before its first.
	LatestRun *RepositoryAgentRun `json:"latest_run"`
	// LatestReading is the latest finished reading in the repository, or
	// nil before one exists.
	LatestReading *AgentReading `json:"latest_reading"`
}

// RepositoryAgentRun is the latest run of a RepositoryAgent.
type RepositoryAgentRun struct {
	// ID identifies the run, for RunsService.GetInRepo.
	ID string `json:"id"`
	// Status is the run's status.
	Status RunStatus `json:"status"`
	// Trigger is why the run started.
	Trigger RunTrigger `json:"trigger"`
	// TrailID and ChangeID are the Change it reviewed.
	TrailID  string `json:"trail_id"`
	ChangeID string `json:"change_id"`
	// HeadSHA is the commit it reviewed; empty before the run resolved it.
	HeadSHA string `json:"head_sha"`
	// Error is why the run failed, timed out, or was canceled by Entire;
	// nil otherwise.
	Error *RunError `json:"error"`
	// CreatedAt is when the run was admitted.
	CreatedAt Timestamp `json:"created_at"`
}

// AgentReading is an agent's signal reading for a Change. One of the value
// fields is set, by the signal's SignalValueType.
type AgentReading struct {
	PercentValue *int    `json:"percent_value"`
	SizeValue    *string `json:"size_value"`
	BooleanValue *bool   `json:"boolean_value"`
	// Rationale explains the reading, and Summary sums up the review.
	Rationale string `json:"rationale"`
	Summary   string `json:"summary"`
	// RunID is the run that produced the reading, and HeadSHA the commit
	// it reviewed.
	RunID   string `json:"run_id"`
	HeadSHA string `json:"head_sha"`
	// UpdatedAt is when the reading was recorded.
	UpdatedAt Timestamp `json:"updated_at"`
}

// ListInRepo returns the active agents of the repository's project that
// review the repository, sorted by name, paused ones included. It needs pull
// on the repository and nothing of the project. It is not paginated.
func (s *AgentsService) ListInRepo(ctx context.Context, repoID string, reqOpts ...RequestOption) (*Response[[]RepositoryAgent], error) {
	req := repoRequest(http.MethodGet, repoID, nil, "/repos/%s/agents", repoID)
	return listCursor[RepositoryAgent](ctx, s.client, req, reqOpts)
}
