package entire

import (
	"context"
	"iter"
	"net/http"
)

// OrgsService reads organizations, their members, and their projects.
type OrgsService service

// Org is an organization: a group of accounts that owns projects.
type Org struct {
	// ID identifies the organization; pass it to OrgsService methods as
	// orgID.
	ID string `json:"id"`
	// Name is the organization's name.
	Name string `json:"name"`
	// Region is where the organization's data lives, such as "us".
	Region string `json:"region"`
	// Provider is "github" when the organization is linked to a GitHub
	// organization, and empty otherwise.
	Provider string `json:"provider"`
	// CreatedAt is when the organization was created.
	CreatedAt Timestamp `json:"createdAt"`
	// Capabilities says what the caller may do with the organization.
	Capabilities OrgCapabilities `json:"capabilities"`
}

// OrgCapabilities says what the caller may do with an organization.
type OrgCapabilities struct {
	// CanCreateProject reports whether the caller may create projects that
	// the organization owns.
	CanCreateProject bool `json:"canCreateProject"`
	// CanManageMembers reports whether the caller may manage the
	// organization's members.
	CanManageMembers bool `json:"canManageMembers"`
	// CanChangeOwners reports whether the caller may change the
	// organization's owners. Each change is still checked when it is made.
	CanChangeOwners bool `json:"canChangeOwners"`
	// CanDelete reports whether the caller may delete the organization.
	CanDelete bool `json:"canDelete"`
	// CanView reports whether the caller may see the organization's
	// projects and members.
	CanView bool `json:"canView"`
	// CanListProjects reports whether the caller may list the
	// organization's projects.
	CanListProjects bool `json:"canListProjects"`
	// CanManageBilling reports whether the caller may view and manage the
	// organization's billing.
	CanManageBilling bool `json:"canManageBilling"`
}

// OrgMember is an organization's member.
type OrgMember struct {
	// ID identifies the membership, not the member's account.
	ID string `json:"id"`
	// OrgID identifies the organization.
	OrgID string `json:"orgId"`
	// AccountID identifies the member's account.
	AccountID string `json:"accountId"`
	// Username is the member's Entire username, or empty until they claim
	// one.
	Username string `json:"username"`
	// Handle is the member's handle for display, prefixed with Provider,
	// such as "github:octocat". It is empty when the member has no handle
	// that can be shown; only GitHub handles are.
	Handle string `json:"handle"`
	// DisplayName is the member's account-level public display name.
	DisplayName string `json:"displayName"`
	// AvatarURL is the URL of the member's public avatar image.
	AvatarURL string `json:"avatarUrl"`
	// Provider is the member's sign-in provider, such as "github".
	Provider string `json:"provider"`
	// Role is the member's role: "owner", "admin", or "member".
	Role string `json:"role"`
	// Status is the membership's state: "active", "inactive", or
	// "pending".
	Status string `json:"status"`
	// CreatedAt is when the membership was created.
	CreatedAt Timestamp `json:"createdAt"`
}

// OrgMemberListOptions orders OrgsService.ListMembers.
type OrgMemberListOptions struct {
	// Sort orders the members by "name" (the default), "role", "status",
	// or "joined". "name" sorts by display name, else handle; "role" puts
	// owners first, then sorts by name; "status" puts active members
	// before pending and inactive ones; "joined" puts the oldest
	// memberships first.
	Sort string `url:"sort"`
	// Order is "asc" (the default) or "desc", which reverses Sort's order.
	// People without a name stay last when sorting by "name".
	Order string `url:"order"`
	ListOptions
}

// List returns one page of the organizations that the caller can see.
func (s *OrgsService) List(ctx context.Context, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]Org], error) {
	req := coreRequest(http.MethodGet, nil, "/orgs")
	Deref(opts).add(req.Query, "pageSize", "pageToken")
	type envelope struct {
		Orgs          []Org  `json:"orgs"`
		NextPageToken string `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[Org] {
		return page[Org]{items: e.Orgs, next: e.NextPageToken}
	})
}

// ListAll iterates over every organization that List returns, page by page.
func (s *OrgsService) ListAll(ctx context.Context, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[Org, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Org], error) {
		return s.List(ctx, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// Get returns an organization.
func (s *OrgsService) Get(ctx context.Context, orgID string, reqOpts ...RequestOption) (*Response[Org], error) {
	return Call[Org](ctx, s.client, coreRequest(http.MethodGet, nil, "/orgs/%s", orgID), reqOpts...)
}

// ListMembers returns one page of an organization's members.
func (s *OrgsService) ListMembers(ctx context.Context, orgID string, opts *OrgMemberListOptions, reqOpts ...RequestOption) (*Response[[]OrgMember], error) {
	req := coreRequest(http.MethodGet, nil, "/orgs/%s/members", orgID)
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "pageSize", "pageToken")
	}
	type envelope struct {
		Members       []OrgMember `json:"members"`
		NextPageToken string      `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[OrgMember] {
		return page[OrgMember]{items: e.Members, next: e.NextPageToken}
	})
}

// ListMembersAll iterates over every member that ListMembers returns, page by
// page.
func (s *OrgsService) ListMembersAll(ctx context.Context, orgID string, opts *OrgMemberListOptions, reqOpts ...RequestOption) iter.Seq2[OrgMember, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]OrgMember], error) {
		p := o
		p.PageToken = token
		return s.ListMembers(ctx, orgID, &p, reqOpts...)
	})
}

// ListProjects returns one page of the projects that an organization owns.
func (s *OrgsService) ListProjects(ctx context.Context, orgID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]Project], error) {
	req := coreRequest(http.MethodGet, nil, "/orgs/%s/projects", orgID)
	Deref(opts).add(req.Query, "pageSize", "pageToken")
	return list(ctx, s.client, req, reqOpts, func(e *projectPage) page[Project] {
		return page[Project]{items: e.Projects, next: e.NextPageToken}
	})
}

// ListProjectsAll iterates over every project that ListProjects returns,
// page by page.
func (s *OrgsService) ListProjectsAll(ctx context.Context, orgID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[Project, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Project], error) {
		return s.ListProjects(ctx, orgID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}
