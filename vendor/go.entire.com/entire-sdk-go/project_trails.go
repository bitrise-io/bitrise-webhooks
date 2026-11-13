package entire

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"strings"
)

// TrailsService manages project Trails. A Trail holds the intent of a piece
// of work: its title, description, type, priority, and assignees. Its
// Changes are the repository branches that carry the work, which
// ChangesService addresses by repository ID and number. Trails are addressed
// by project path, such as "et/acme", and Trail ID or number.
type TrailsService service

// TrailStatus is a project Trail's status.
type TrailStatus string

// Trail statuses.
const (
	TrailStatusOpen   TrailStatus = "open"
	TrailStatusClosed TrailStatus = "closed"
)

// TrailPhase is a project Trail's progress, derived from its status and its
// Changes.
type TrailPhase string

// Trail phases.
const (
	// TrailPhasePlanning is an open Trail with no Changes, or whose
	// Changes have no code and are not ready for review.
	TrailPhasePlanning TrailPhase = "planning"
	// TrailPhaseBuilding is an open Trail with a Change that has code, is
	// ready for review, or has ended.
	TrailPhaseBuilding TrailPhase = "building"
	// TrailPhaseComplete is an open Trail whose Changes have all merged or
	// closed.
	TrailPhaseComplete TrailPhase = "complete"
	// TrailPhaseClosed is a closed Trail.
	TrailPhaseClosed TrailPhase = "closed"
)

// TrailType is the kind of work a Trail is.
type TrailType string

// Trail types.
const (
	TrailTypeBug     TrailType = "bug"
	TrailTypeFeature TrailType = "feature"
	TrailTypeTask    TrailType = "task"
)

// TrailPriority is a Trail's priority.
type TrailPriority string

// Trail priorities.
const (
	TrailPriorityUrgent TrailPriority = "urgent"
	TrailPriorityHigh   TrailPriority = "high"
	TrailPriorityMedium TrailPriority = "medium"
	TrailPriorityLow    TrailPriority = "low"
	TrailPriorityNone   TrailPriority = "none"
)

// TrailSort orders a listing of Trails. Each order breaks ties by the most
// recent activity.
type TrailSort string

// Trail orders.
const (
	TrailSortUpdated  TrailSort = "updated"
	TrailSortTitle    TrailSort = "title"
	TrailSortStatus   TrailSort = "status"
	TrailSortType     TrailSort = "type"
	TrailSortPriority TrailSort = "priority"
	TrailSortAssignee TrailSort = "assignee"
)

// ProjectTrail is a project Trail. Fields documented as set by some methods
// are empty in the others.
type ProjectTrail struct {
	// ID identifies the Trail. It never changes.
	ID string `json:"id"`
	// ProjectID is the ID of the project that holds the Trail.
	ProjectID string `json:"projectId"`
	// Number is the Trail's number within its project. The TrailsService
	// methods take it, as a decimal string, in place of the ID.
	Number int64 `json:"number"`
	// URL is the Trail's page on the web, or empty when the server has no
	// web address configured.
	URL string `json:"url"`
	// Title is the Trail's title.
	Title string `json:"title"`
	// Body is the description: the Markdown it was written in, or, once it
	// has been edited as rich text, a plain-text rendering of it.
	Body string `json:"body"`
	// BodyDocument is the description as rich text. GetInProject and
	// UpdateInProject set it.
	BodyDocument *BodyDocument `json:"bodyDocument"`
	// Status is TrailStatusOpen or TrailStatusClosed.
	Status TrailStatus `json:"status"`
	// Phase is the Trail's progress.
	Phase TrailPhase `json:"phase"`
	// Type is the kind of work the Trail is.
	Type TrailType `json:"type"`
	// Priority is the Trail's priority.
	Priority TrailPriority `json:"priority"`
	// Metadata holds the caller-defined fields set with
	// ProjectTrailCreateRequest.Metadata or ProjectTrailUpdateRequest.
	Metadata map[string]any `json:"metadata"`
	// Assignees are the people the Trail is assigned to, as logins.
	Assignees []string `json:"assignees"`
	// AssigneeAccountIDs are the account IDs of Assignees, in the same
	// order. An entry is empty for an assignee with no account.
	AssigneeAccountIDs []string `json:"assigneeAccountIds"`
	// AssigneeProfiles describe the assignees that have accounts.
	AssigneeProfiles []TrailPerson `json:"assigneeProfiles"`
	// RepoIDs are the repositories the Trail is scoped to that the caller
	// can see.
	RepoIDs []string `json:"repoIds"`
	// AuthorAccountID is the account that opened the Trail: a person's
	// account ID, or "svc_" and an ID for a service account. It is empty
	// when an automation, such as a plugin, opened the Trail, and for work
	// imported from a forge author with no account.
	AuthorAccountID string `json:"authorAccountId"`
	// Author describes the account that opened the Trail, or is nil when
	// AuthorAccountID is empty.
	Author *TrailPerson `json:"author"`
	// Changes are the Trail's Changes that the caller can read.
	// ListInProject and Me.ListProjectTrails list them only for a Trail in
	// TrailPhaseBuilding.
	Changes []ProjectTrailChange `json:"changes"`
	// Readiness rolls up the readiness of Changes: "ready" when all are
	// ready to merge, "in_review", "none", or "unknown". ListInProject
	// sets it.
	Readiness string `json:"readiness"`
	// Capabilities say what the caller may do with the Trail.
	Capabilities *TrailCapabilities `json:"capabilities"`
	// Project describes the Trail's project. GetInProject,
	// UpdateInProject, and Me.ListProjectTrails set it.
	Project *TrailProject `json:"project"`
	// IsPossiblyPartial reports that RepoIDs and Changes may leave out
	// what the caller cannot see. It does not mean anything was left out.
	IsPossiblyPartial bool `json:"isPossiblyPartial"`
	// CreatedAt is when the Trail was opened.
	CreatedAt Timestamp `json:"createdAt"`
	// MetadataUpdatedAt is when the Trail's own fields last changed. Use
	// the response's ETag header, not this, for IfMatch.
	MetadataUpdatedAt Timestamp `json:"metadataUpdatedAt"`
	// LastActivityAt is when anything meaningful last happened on the
	// Trail or a Change the caller can read.
	LastActivityAt Timestamp `json:"lastActivityAt"`
	// LastVisitedAt is when the caller last viewed the Trail in the web
	// app, or zero.
	LastVisitedAt Timestamp `json:"lastVisitedAt"`
	// ClosedAt is when the Trail was closed; zero while it is open.
	ClosedAt Timestamp `json:"closedAt"`
}

// TrailPerson describes a person, or a service account, on a Trail.
type TrailPerson struct {
	// AccountID is the account's ID: a person's account ID, or "svc_" and
	// an ID for a service account.
	AccountID string `json:"accountId"`
	// Username is the account's Entire username, or empty.
	Username string `json:"username"`
	// Login is the account's GitHub login, or empty.
	Login string `json:"login"`
	// DisplayName is the account's display name, or empty.
	DisplayName string `json:"displayName"`
	// AvatarURL is the URL of the account's avatar image, or empty.
	AvatarURL string `json:"avatarUrl"`
	// Identity describes the account for display.
	Identity Identity `json:"identity"`
}

// TrailCapabilities say what the caller may do with a Trail.
type TrailCapabilities struct {
	// CanEdit reports that the caller may edit the Trail's fields.
	CanEdit bool `json:"canEdit"`
	// CanManageScope reports that the caller may change the Trail's
	// repositories and move Changes between Trails, which takes project
	// management.
	CanManageScope bool `json:"canManageScope"`
}

// TrailProject describes the project that holds a Trail.
type TrailProject struct {
	// ID is the project's ID.
	ID string
	// Name is the project's name.
	Name string
	// Provider is "github" or "entire".
	Provider string
	// Host is the first element of the project's path: "gh" or "et".
	Host string
	// PublicName is the second element of the project's path, such as
	// "acme".
	PublicName string
	// Jurisdiction is the region that stores the project, such as "us".
	Jurisdiction string
	// CanManageTrails reports that the caller may manage the project's
	// Trails.
	CanManageTrails bool
}

// Path returns the project's path, such as "et/acme", which the
// TrailsService methods take.
func (p TrailProject) Path() string { return p.Host + "/" + p.PublicName }

// UnmarshalJSON decodes a project as Trail responses carry it.
func (p *TrailProject) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Provider     string `json:"provider"`
		Jurisdiction string `json:"jurisdiction"`
		Reference    struct {
			Forge   string `json:"forge"`
			Project string `json:"project"`
		} `json:"reference"`
		Capabilities struct {
			CanManageTrails bool `json:"canManageTrails"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*p = TrailProject{
		ID: wire.ID, Name: wire.Name, Provider: wire.Provider, Jurisdiction: wire.Jurisdiction,
		Host: wire.Reference.Forge, PublicName: wire.Reference.Project, CanManageTrails: wire.Capabilities.CanManageTrails,
	}
	return nil
}

// ProjectTrailChange is a Change of a project Trail, in summary.
type ProjectTrailChange struct {
	// ID is the Change's ID.
	ID string `json:"id"`
	// RepositoryID is the repository the Change is in.
	RepositoryID string `json:"repositoryId"`
	// Repository is the repository's name, or empty when it has not been
	// resolved.
	Repository string `json:"repository"`
	// Number is the Change's number in its repository: pass it, with
	// RepositoryID, to the ChangesService methods.
	Number int64 `json:"number"`
	// Title is the Change's title.
	Title string `json:"title"`
	// Branch is the Change's branch, or empty when it has none, and
	// OriginalBranch the branch it was opened with.
	Branch         string `json:"branch"`
	OriginalBranch string `json:"originalBranch"`
	// Base is the branch the Change merges into.
	Base string `json:"base"`
	// MergedBase is the branch a merged Change landed on, as Change.MergedBase
	// describes.
	MergedBase string `json:"mergedBase"`
	// PullRequestNumber is the number of the GitHub pull request linked to
	// the Change, or zero.
	PullRequestNumber int64 `json:"ghPrId"`
	// Status is the Change's lifecycle status.
	Status ChangeStatus `json:"status"`
	// HasCodeChanges reports whether the branch differs from its base.
	HasCodeChanges bool `json:"hasCodeChanges"`
	// ReadyForReview reports that the Change is marked ready for review
	// and is not yet ready to merge. Me.ListProjectTrails sets it.
	ReadyForReview bool `json:"readyForReview"`
	// Readiness is "ready" to merge, "in_review", "none", or "unknown",
	// from stored gate, conflict, and CI evidence.
	Readiness string `json:"readiness"`
	// Mergeability is the stored conflict evidence behind Readiness, not a
	// live merge verdict (see ChangesService.Mergeability). It is nil for a
	// merged or closed Change, or when the evidence is unavailable.
	Mergeability *StoredMergeability `json:"mergeability"`
	// Deployments count the deployments of the Change's head commit.
	// GetInProject and UpdateInProject set them.
	Deployments *DeploymentSummary `json:"deployments"`
	// CreatedAt is when the Change was opened, UpdatedAt when it last
	// changed, and LastActivityAt when anything meaningful last happened
	// on it.
	CreatedAt      Timestamp `json:"createdAt"`
	UpdatedAt      Timestamp `json:"updatedAt"`
	LastActivityAt Timestamp `json:"lastActivityAt"`
}

// StoredMergeability is the stored conflict evidence of a Change.
type StoredMergeability struct {
	// ConflictStatus is "clean", "conflicting", or "unknown".
	ConflictStatus string `json:"conflict_status"`
	// HeadSHA is the commit the evidence is for, which may not be the
	// Change's current head.
	HeadSHA string `json:"head_sha"`
	// RefreshedAt is when the evidence was last refreshed, or zero.
	RefreshedAt Timestamp `json:"refreshed_at"`
	// ForgeMergeBlock is why the forge refuses the merge, as
	// ForgeMergeBlock.Reason describes, or empty.
	ForgeMergeBlock string `json:"forge_merge_block"`
	// Mergeable reports that the evidence establishes the Change is ready
	// to merge. Me.ListProjectTrails sets it; the merge itself decides.
	Mergeable bool `json:"mergeable"`
}

// ProjectTrailCreateRequest is the body of TrailsService.CreateInProject.
type ProjectTrailCreateRequest struct {
	// Title is the Trail's title. It is required and must not be blank.
	Title string `json:"title"`
	// Body is the description, in Markdown.
	Body string `json:"body,omitempty"`
	// Status is TrailStatusOpen, the default, or TrailStatusClosed.
	Status TrailStatus `json:"status,omitempty"`
	// Type is TrailTypeBug, TrailTypeFeature, or TrailTypeTask, the
	// default.
	Type TrailType `json:"type,omitempty"`
	// Priority is one of the TrailPriority constants; empty selects
	// TrailPriorityNone.
	Priority TrailPriority `json:"priority,omitempty"`
	// Metadata holds caller-defined fields to store with the Trail.
	Metadata map[string]any `json:"metadata,omitempty"`
	// Assignees are actor references: "ulid:<account ID>" or "@<username>".
	// An automation cannot set them.
	Assignees []string `json:"assignees,omitempty"`
	// RepoIDs scope the Trail to repositories. Changes scope it to theirs
	// too.
	RepoIDs []string `json:"repoIds,omitempty"`
	// Changes are the repository branches to open with the Trail.
	Changes []ProjectTrailChangeRequest `json:"changes,omitempty"`
}

// ProjectTrailChangeRequest opens one Change of a project Trail.
type ProjectTrailChangeRequest struct {
	// RepositoryID is the repository. It is required.
	RepositoryID string `json:"repositoryId"`
	// Title is the Change's title; empty uses the Trail's.
	Title string `json:"title,omitempty"`
	// BranchName is the Change's branch, which BranchAction creates or
	// links. It is required.
	BranchName string `json:"branchName,omitempty"`
	// BranchAction is "create", the default, to create BranchName from
	// Base, or "link" to use an existing branch, such as one just pushed.
	// Linking a branch that already has a Change of this Trail returns
	// that Change; one of another Trail is refused.
	BranchAction string `json:"branchAction,omitempty"`
	// Base is the branch to merge into; empty selects the repository's
	// default branch.
	Base string `json:"base,omitempty"`
	// Status is ChangeStatusOpen, the default, or ChangeStatusClosed.
	Status ChangeStatus `json:"status,omitempty"`
	// ReadyForReview marks the Change ready for review.
	ReadyForReview bool `json:"readyForReview,omitempty"`
}

// ProjectTrailUpdateRequest is the body of TrailsService.UpdateInProject.
// Nil fields are left unchanged.
type ProjectTrailUpdateRequest struct {
	// Title sets the Trail's title.
	Title *string `json:"title,omitempty"`
	// Body replaces the description, in Markdown.
	Body *string `json:"body,omitempty"`
	// Status sets TrailStatusOpen or TrailStatusClosed.
	Status *TrailStatus `json:"status,omitempty"`
	// Type sets the kind of work.
	Type *TrailType `json:"type,omitempty"`
	// Priority sets the priority.
	Priority *TrailPriority `json:"priority,omitempty"`
	// Metadata replaces the caller-defined fields; a non-nil empty map
	// clears them.
	Metadata map[string]any `json:"metadata,omitzero"`
	// Assignees replaces the assignees, as actor references: "ulid:<account
	// ID>" or "@<username>". A non-nil empty slice clears them.
	Assignees []string `json:"assignees,omitzero"`
	// RepoIDs replaces the repositories the Trail is scoped to, which takes
	// project management.
	RepoIDs []string `json:"repoIds,omitzero"`
}

// ProjectTrailListOptions filters TrailsService.ListInProject. Each filter
// matches any of its values, each Not filter excludes all of its values, and
// the filters combine.
type ProjectTrailListOptions struct {
	// RepoID selects Trails with a Change in the repository.
	RepoID string `url:"repoId"`
	// Status and StatusNot select Trails by status.
	Status    []TrailStatus `url:"status"`
	StatusNot []TrailStatus `url:"statusNot"`
	// Phase and PhaseNot select Trails by phase.
	Phase    []TrailPhase `url:"phase"`
	PhaseNot []TrailPhase `url:"phaseNot"`
	// Type and TypeNot select Trails by type.
	Type    []TrailType `url:"type"`
	TypeNot []TrailType `url:"typeNot"`
	// Priority and PriorityNot select Trails by priority.
	Priority    []TrailPriority `url:"priority"`
	PriorityNot []TrailPriority `url:"priorityNot"`
	// Author and AuthorNot select Trails by their authors' account IDs.
	Author    []string `url:"author"`
	AuthorNot []string `url:"authorNot"`
	// Assignee and AssigneeNot select Trails by their assignees' account
	// IDs.
	Assignee    []string `url:"assignee"`
	AssigneeNot []string `url:"assigneeNot"`
	// Search matches the Trail's text, its "#number", and its project,
	// ignoring case.
	Search string `url:"search"`
	// Sort orders the Trails; empty selects TrailSortUpdated, most recent
	// activity first.
	Sort TrailSort `url:"sort"`
	ListOptions
}

// projectTrailPath splits a project path such as "et/acme" into its host
// and name.
func projectTrailPath(projectPath string) (host, name string, err error) {
	host, name, ok := strings.Cut(projectPath, "/")
	if !ok || (host != "gh" && host != "et") || name == "" || strings.Contains(name, "/") {
		return "", "", errors.New(`entire: project path must be "gh/<owner>" or "et/<project>"`)
	}
	return host, strings.ToLower(name), nil
}

// projectTrailRequest builds a request for a project Trail route. It reaches
// the cell of repoID when one is known, as the CLI does: finding the project's
// cell asks Core for the project, which a caller granted only a repository,
// such as a service account, may not read.
func projectTrailRequest(method, projectPath, repoID string, body any, suffix string, args ...any) *Request {
	req := projectRequest(method, projectPath, body, "/trails"+suffix, args...)
	if repoID != "" {
		req.Route = Route{RepoID: repoID}
	}
	return req
}

// CreateInProject opens a Trail in the project at projectPath, such as
// "et/acme". The response's ETag header is the Trail's version, for AddChange
// and UpdateInProject.
//
// Opening it with no RepoIDs needs project management, and opening it with
// body.Changes needs project management and a person or service account. A
// repository writer, such as a service account granted one repository, opens
// it with RepoIDs set to that repository and no Changes, then adds its branch
// with AddChange. A plugin installation token, or another automation, can
// open a Trail with RepoIDs it may push to or write Trails in, but fails with
// 403 Forbidden if it sets Assignees or Changes. A repository whose Trails
// setting is off refuses it.
func (s *TrailsService) CreateInProject(ctx context.Context, projectPath string, body ProjectTrailCreateRequest, reqOpts ...RequestOption) (*Response[ProjectTrail], error) {
	repoID := ""
	switch {
	case len(body.Changes) > 0:
		repoID = body.Changes[0].RepositoryID
	case len(body.RepoIDs) > 0:
		repoID = body.RepoIDs[0]
	}
	req := projectTrailRequest(http.MethodPost, projectPath, repoID, body, "")
	return Call[ProjectTrail](ctx, s.client, req, reqOpts...)
}

// ProjectTrailChangeRef identifies a Change that AddChange created or
// linked.
type ProjectTrailChangeRef struct {
	// ID identifies the Change.
	ID string `json:"id"`
	// TrailID is the project Trail the Change belongs to.
	TrailID string `json:"trailId"`
	// RepositoryID is the Change's repository.
	RepositoryID string `json:"repositoryId"`
}

// AddChange adds a Change to the project Trail trailID: it creates
// body.BranchName, or links an existing branch with BranchAction "link".
// etag is the Trail's version, the ETag header of the response that created
// or last read it; the call fails with 412 Precondition Failed if the Trail
// has changed since. The response's ETag header is the Trail's new version.
// Read the Change with GetChange, or the Trail with GetInProject, to see its
// number.
//
// The caller needs to be able to edit the Trail and write to
// body.RepositoryID, which it adds to the Trail's repositories. Only a person
// or a service account can add a Change: a plugin installation token, or
// another automation, fails with 403 Forbidden.
func (s *TrailsService) AddChange(ctx context.Context, projectPath, trailID, etag string, body ProjectTrailChangeRequest, reqOpts ...RequestOption) (*Response[ProjectTrailChangeRef], error) {
	req := projectTrailRequest(http.MethodPost, projectPath, body.RepositoryID, body, "/%s/changes", trailID)
	req.Header.Set("If-Match", etag)
	if etag == "" && req.invalid == nil {
		req.invalid = errors.New("entire: etag must not be empty; pass the ETag of the response that created or last read the Trail")
	}
	req.Header.Set("Idempotency-Key", NewIdempotencyKey())
	return Call[ProjectTrailChangeRef](ctx, s.client, req, reqOpts...)
}

// GetChange returns the Change changeID of the project Trail trailID, with
// the detail that ChangesService.Get returns and the Change's TrailID,
// RepositoryID, and Repository. It fails with 404 Not Found unless the
// caller can read the Change's repository. A caller granted only a repository
// passes WithRoute(Route{RepoID: ...}).
func (s *TrailsService) GetChange(ctx context.Context, projectPath, trailID, changeID string, opts *ChangeGetOptions, reqOpts ...RequestOption) (*Response[Change], error) {
	req := projectTrailRequest(http.MethodGet, projectPath, "", nil, "/%s/changes/%s", trailID, changeID)
	addQuery(req.Query, opts)
	return Call[Change](ctx, s.client, req, reqOpts...)
}

// AttachChange moves the existing Change changeID into the project Trail
// trailID, from the Trail that owns it now. The Change's repository must be
// in the Trail's repositories. The caller needs write access to the project
// and to the repository; only a person or a service account can attach. With IfMatch and the Trail's ETag, it fails with 412 Precondition
// Failed if the Trail has changed since. The response's ETag header is the
// Trail's new version.
func (s *TrailsService) AttachChange(ctx context.Context, projectPath, trailID, changeID string, reqOpts ...RequestOption) (*Response[ProjectTrailChange], error) {
	req := projectTrailRequest(http.MethodPut, projectPath, "", nil, "/%s/changes/%s", trailID, changeID)
	return Call[ProjectTrailChange](ctx, s.client, req, reqOpts...)
}

// GetInProject returns the Trail trailID of the project at projectPath.
// trailID is the Trail's ID or its number. Its ETag header is the Trail's
// version, for AddChange and UpdateInProject. A caller granted only a
// repository passes WithRoute(Route{RepoID: ...}).
func (s *TrailsService) GetInProject(ctx context.Context, projectPath, trailID string, reqOpts ...RequestOption) (*Response[ProjectTrail], error) {
	req := projectTrailRequest(http.MethodGet, projectPath, "", nil, "/%s", trailID)
	return Call[ProjectTrail](ctx, s.client, req, reqOpts...)
}

// UpdateInProject changes a project Trail's intent. With IfMatch and the
// ETag header of an earlier response, it fails with 412 Precondition Failed
// if the Trail has changed since. An automation, such as a plugin
// installation token, can update a Trail it may edit, but fails with 403
// Forbidden if it sets Assignees. A caller granted only a repository passes
// WithRoute(Route{RepoID: ...}).
func (s *TrailsService) UpdateInProject(ctx context.Context, projectPath, trailID string, body ProjectTrailUpdateRequest, reqOpts ...RequestOption) (*Response[ProjectTrail], error) {
	req := projectTrailRequest(http.MethodPatch, projectPath, "", body, "/%s", trailID)
	return Call[ProjectTrail](ctx, s.client, req, reqOpts...)
}

// ListInProject returns one page of the project's Trails, in opts.Sort
// order. With opts.RepoID it reaches that repository's cell, so a caller
// granted only the repository can list its Trails.
func (s *TrailsService) ListInProject(ctx context.Context, projectID string, opts *ProjectTrailListOptions, reqOpts ...RequestOption) (*Response[[]ProjectTrail], error) {
	route := Route{ProjectID: projectID}
	if opts != nil && opts.RepoID != "" {
		route = Route{RepoID: opts.RepoID}
	}
	req := newRequest(http.MethodGet, route, false, nil, "/trails", nil)
	req.Query.Set("projectId", projectID)
	req.Query.Set("sort", string(TrailSortUpdated))
	req.Query.Set("groupBy", "none")
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "pageSize", "pageToken")
	}
	type envelope struct {
		Items         []ProjectTrail `json:"items"`
		NextPageToken string         `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[ProjectTrail] {
		return page[ProjectTrail]{items: e.Items, next: e.NextPageToken}
	})
}

// ListInProjectAll iterates over every Trail that ListInProject returns,
// page by page.
func (s *TrailsService) ListInProjectAll(ctx context.Context, projectID string, opts *ProjectTrailListOptions, reqOpts ...RequestOption) iter.Seq2[ProjectTrail, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]ProjectTrail], error) {
		p := o
		p.PageToken = token
		return s.ListInProject(ctx, projectID, &p, reqOpts...)
	})
}

// TrailTimelineEvent is something that happened on a project Trail. Kind
// says what, and which of the other fields are set.
type TrailTimelineEvent struct {
	// ID identifies the event.
	ID string `json:"id"`
	// Kind is "trail_created", "trail_status_changed", "trail_updated",
	// "change_added", "change_removed", or "comment". New kinds may appear.
	Kind string `json:"kind"`
	// Actor is who caused the event, or nil.
	Actor *TimelineActor `json:"actor"`
	// OccurredAt is when the event happened, or zero.
	OccurredAt Timestamp `json:"occurred_at"`
	// FromStatus and ToStatus are the statuses of a "trail_status_changed"
	// event. FromStatus is empty when unknown.
	FromStatus TrailStatus `json:"from_status"`
	ToStatus   TrailStatus `json:"to_status"`
	// Fields name the fields a "trail_updated" event changed.
	Fields []string `json:"fields"`
	// ChangeID, RepositoryID, and Branch identify the Change of a
	// "change_added" or "change_removed" event, and Reason says why a
	// Change was removed, or is empty.
	ChangeID     string `json:"change_id"`
	RepositoryID string `json:"repository_id"`
	Branch       string `json:"branch"`
	Reason       string `json:"reason"`
	// CommentID, DiscussionID, and IsReply identify the message of a
	// "comment" event.
	CommentID    string `json:"comment_id"`
	DiscussionID string `json:"discussion_id"`
	IsReply      bool   `json:"is_reply"`
}

// TimelineActor is who caused a timeline event.
type TimelineActor struct {
	// ActorID identifies the actor, such as an account ID, or is empty.
	ActorID string `json:"actorId"`
	// Login is the actor's forge login, or empty.
	Login string `json:"login"`
	// Label names an actor that has no account, such as an automation, or
	// is empty.
	Label string `json:"label"`
	// Identity describes the actor for display.
	Identity Identity `json:"identity"`
}

// ListTimeline returns one page of the project Trail's timeline, newest
// first: its creation, status changes, edits, Changes added and removed, and
// discussion messages. It leaves out the Changes of repositories the caller
// cannot read.
func (s *TrailsService) ListTimeline(ctx context.Context, projectPath, trailID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]TrailTimelineEvent], error) {
	req := projectTrailRequest(http.MethodGet, projectPath, "", nil, "/%s/timeline", trailID)
	Deref(opts).add(req.Query, "per_page", "cursor")
	type envelope struct {
		Items      []TrailTimelineEvent `json:"items"`
		NextCursor string               `json:"next_cursor"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[TrailTimelineEvent] {
		return page[TrailTimelineEvent]{items: e.Items, next: e.NextCursor}
	})
}

// ListTimelineAll iterates over every event that ListTimeline returns, page
// by page.
func (s *TrailsService) ListTimelineAll(ctx context.Context, projectPath, trailID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[TrailTimelineEvent, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]TrailTimelineEvent], error) {
		return s.ListTimeline(ctx, projectPath, trailID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}
