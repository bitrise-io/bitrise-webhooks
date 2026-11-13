package entire

import (
	"context"
	"iter"
	"net/http"
	"time"
)

// MeService reads your account, your project Trails across projects, and
// your sessions across repositories.
type MeService service

// Account is the account you are signed in as.
type Account struct {
	// ID identifies the account.
	ID string
	// Handle is the account's primary handle, such as its GitHub login.
	Handle string
	// Username is the account's Entire username, or empty until you claim
	// one.
	Username string
	// DisplayName is the name your sign-in provider reports. You can't edit
	// it; Profile.DisplayName is the name you can. It is empty until the
	// provider supplies one.
	DisplayName string
	// AvatarURL is the URL of the account's avatar image.
	AvatarURL string
	// HomeJurisdiction is the region that stores your data, such as "us" or
	// "eu".
	HomeJurisdiction string
	// CreatedAt is when the account was created.
	CreatedAt Timestamp
	// Handles are the sign-in identities linked to the account.
	Handles []AccountHandle
	// Auth is the sign-in identity that authenticated this session.
	Auth AccountAuth
	// Deletion is set while the account is scheduled for deletion, and nil
	// otherwise.
	Deletion *AccountDeletion
	// Profile is your editable profile. It is nil when the Core that
	// answered is not your home region's, which keeps profiles, and then
	// ProfileUnavailable says where it is. It is also nil when you have no
	// profile yet.
	Profile *Profile
	// ProfileUnavailable is set when the Core that answered is not your home
	// region's, so Profile is nil. Read your account from the Core at its
	// HomeCoreURL, with a client made with WithCoreURL, to get Profile.
	ProfileUnavailable *ProfileUnavailable
	// Mode is how the Core that answered is deployed: "global" or
	// "regional". It is empty against a Core that doesn't report it.
	Mode string
	// Jurisdiction is the region of the Core that answered, such as "us".
	// It is empty for a global Core.
	Jurisdiction string
}

// AccountAuth is the sign-in identity that authenticated a session.
type AccountAuth struct {
	// Provider is the identity provider, such as "github" or "google".
	Provider string `json:"provider"`
	// ProviderUserID is the provider's ID for the user.
	ProviderUserID string `json:"providerUserId"`
}

// AccountDeletion is an account's pending deletion.
type AccountDeletion struct {
	// RequestedAt is when the deletion was requested.
	RequestedAt Timestamp `json:"requestedAt"`
	// CooldownEndsAt is when the cooldown ends. Until then the deletion can
	// be canceled; after it, Entire may delete the account for good.
	CooldownEndsAt Timestamp `json:"cooldownEndsAt"`
}

// ProfileUnavailable says why Account.Profile is nil: the Core that answered
// is not your home region's.
type ProfileUnavailable struct {
	// Reason is "foreign_jurisdiction".
	Reason string `json:"error"`
	// Jurisdiction is your home region, such as "eu".
	Jurisdiction string `json:"jurisdiction"`
	// HomeCoreURL is the origin of your home region's Core, such as
	// "https://eu.auth.entire.io". Pass it to WithCoreURL to read your
	// profile there. It is an API origin, not a page to open in a browser.
	HomeCoreURL string `json:"homeCoreUrl"`
	// Message explains the situation, ready to show to a person.
	Message string `json:"message"`
}

// AccountHandle is a sign-in identity linked to an account, such as a GitHub
// login.
type AccountHandle struct {
	// Provider is the identity provider, such as "github".
	Provider string `json:"provider"`
	// Handle is the login at the provider.
	Handle string `json:"handle"`
	// ProviderUserID is the provider's ID for the user.
	ProviderUserID string `json:"providerUserId"`
	// Email is the provider's public profile email, which may be empty. It
	// is not the account's contact email.
	Email string `json:"email"`
}

// Profile is the editable part of an account.
type Profile struct {
	// DisplayName is the display name you can edit, unlike
	// Account.DisplayName.
	DisplayName string `json:"displayName"`
	// Email is the profile's email address.
	Email string `json:"email"`
	// Bio is the profile's free-form biography.
	Bio string `json:"bio"`
	// Company is the company the profile names.
	Company string `json:"company"`
	// Location is the location the profile gives.
	Location string `json:"location"`
	// ProviderEmails maps each linked sign-in provider, such as "google",
	// to the primary email that provider gave at login. GitHub is not
	// included: its public email is on Account.Handles. It is empty when no
	// linked provider gave one.
	ProviderEmails map[string]string `json:"providerEmails"`
}

// Get returns your account.
func (s *MeService) Get(ctx context.Context, reqOpts ...RequestOption) (*Response[Account], error) {
	type me struct {
		Global struct {
			AccountID        string          `json:"accountId"`
			Handle           string          `json:"handle"`
			Username         string          `json:"username"`
			DisplayName      string          `json:"displayName"`
			AvatarURL        string          `json:"avatarUrl"`
			HomeJurisdiction string          `json:"homeJurisdiction"`
			CreatedAt        Timestamp       `json:"createdAt"`
			Handles          []AccountHandle `json:"handles"`
		} `json:"global"`
		Regional            *Profile            `json:"regional"`
		RegionalUnavailable *ProfileUnavailable `json:"regionalUnavailable"`
		Deletion            *AccountDeletion    `json:"deletion"`
		Auth                AccountAuth         `json:"auth"`
		Mode                string              `json:"mode"`
		Jurisdiction        string              `json:"jurisdiction"`
	}
	return unwrap(ctx, s.client, coreRequest(http.MethodGet, nil, "/me"), reqOpts, func(m *me) Account {
		return Account{
			ID:                 m.Global.AccountID,
			Handle:             m.Global.Handle,
			Username:           m.Global.Username,
			DisplayName:        m.Global.DisplayName,
			AvatarURL:          m.Global.AvatarURL,
			HomeJurisdiction:   m.Global.HomeJurisdiction,
			CreatedAt:          m.Global.CreatedAt,
			Handles:            m.Global.Handles,
			Auth:               m.Auth,
			Deletion:           m.Deletion,
			Profile:            m.Regional,
			ProfileUnavailable: m.RegionalUnavailable,
			Mode:               m.Mode,
			Jurisdiction:       m.Jurisdiction,
		}
	})
}

// MeListOptions selects the window of MeService.ListSessions.
// The default window is the last six months.
type MeListOptions struct {
	// Timeframe is a window ending now: "last-month" (30 days),
	// "last-3-months", or "last-6-months". Empty selects "last-6-months".
	Timeframe string `url:"timeframe"`
	// Since starts the window instead of Timeframe. The server moves a Since
	// more than a year before the window's end to one year before it.
	Since time.Time `url:"since"`
	// Until ends the window instead of now.
	Until time.Time `url:"until"`
	// Limit caps the number of results, from 1 to 1000; zero selects 500.
	Limit int `url:"limit"`
}

// MeSessions are your sessions across your repositories.
type MeSessions struct {
	// Sessions are the sessions, most recently active first.
	Sessions []Session
	// Partial lists why Sessions may be incomplete: "unroutable_repos" when
	// some of your repositories could not be reached. It is empty for a
	// complete list.
	Partial []string
}

// meta is the envelope member that marks a partial result.
type meta struct {
	Partial []string `json:"partial"`
}

// ListSessions returns your agent sessions across your repositories, most
// recently active first. Each session's RepoFullName names its repository.
func (s *MeService) ListSessions(ctx context.Context, opts *MeListOptions, reqOpts ...RequestOption) (*Response[MeSessions], error) {
	req := homeRequest(http.MethodGet, nil, "/me/sessions")
	addQuery(req.Query, opts)
	type envelope struct {
		Sessions []Session `json:"sessions"`
		Meta     meta      `json:"meta"`
	}
	return unwrap(ctx, s.client, req, reqOpts, func(e *envelope) MeSessions {
		return MeSessions{Sessions: e.Sessions, Partial: e.Meta.Partial}
	})
}

// MeProjectTrailListOptions filters MeService.ListProjectTrails. Each filter
// matches any of its values, each Not filter excludes all of its values, and
// the filters combine.
type MeProjectTrailListOptions struct {
	// ProjectIDs selects Trails in these projects.
	ProjectIDs []string `url:"project_id"`
	// Status and StatusNot select Trails by status.
	Status    []TrailStatus `url:"status"`
	StatusNot []TrailStatus `url:"status_not"`
	// Phase and PhaseNot select Trails by phase.
	Phase    []TrailPhase `url:"phase"`
	PhaseNot []TrailPhase `url:"phase_not"`
	// Type and TypeNot select Trails by type.
	Type    []TrailType `url:"type"`
	TypeNot []TrailType `url:"type_not"`
	// Priority and PriorityNot select Trails by priority.
	Priority    []TrailPriority `url:"priority"`
	PriorityNot []TrailPriority `url:"priority_not"`
	// Author and AuthorNot select Trails by their authors' account IDs.
	Author    []string `url:"author"`
	AuthorNot []string `url:"author_not"`
	// Assignee and AssigneeNot select Trails by their assignees' account
	// IDs.
	Assignee    []string `url:"assignee"`
	AssigneeNot []string `url:"assignee_not"`
	// Search matches the Trail's title, its "#number", and its project,
	// ignoring case.
	Search string `url:"search"`
	// Sort orders the Trails; empty selects TrailSortUpdated, most recent
	// activity first.
	Sort TrailSort `url:"sort"`
	ListOptions
}

// MeProjectTrails are one page of your project Trails.
type MeProjectTrails struct {
	// Trails are the Trails, in the order the options select.
	Trails []ProjectTrail
	// Jurisdiction is the region whose Trails the page covers, such as
	// "us": your home region.
	Jurisdiction string
	// Partial lists why Trails may be incomplete: "unroutable_projects"
	// when some of your projects could not be reached, and
	// "change_readiness_unavailable" when the Trails' Changes could not be
	// read, which leaves them empty. It is empty for a complete page.
	Partial []string
}

// ListProjectTrails returns one page of the project Trails you opened or are
// assigned to, across the projects you can read in your home region. Each
// Trail's Project names its project, and its Changes are listed only while
// it is in TrailPhaseBuilding. It needs a person's or service account's
// token: an automation fails with 403 Forbidden.
func (s *MeService) ListProjectTrails(ctx context.Context, opts *MeProjectTrailListOptions, reqOpts ...RequestOption) (*Response[MeProjectTrails], error) {
	req := homeRequest(http.MethodGet, nil, "/me/project-trails")
	if opts != nil {
		addQuery(req.Query, opts)
		opts.ListOptions.add(req.Query, "per_page", "cursor")
	}
	type envelope struct {
		Items        []meProjectTrail `json:"items"`
		NextCursor   string           `json:"next_cursor"`
		Jurisdiction string           `json:"jurisdiction"`
		Meta         meta             `json:"meta"`
	}
	next := ""
	resp, err := unwrap(ctx, s.client, req, reqOpts, func(e *envelope) MeProjectTrails {
		next = e.NextCursor
		page := MeProjectTrails{Trails: make([]ProjectTrail, len(e.Items)), Jurisdiction: e.Jurisdiction, Partial: e.Meta.Partial}
		for i, item := range e.Items {
			page.Trails[i] = item.trail()
		}
		return page
	})
	if resp != nil {
		resp.Next = next
	}
	return resp, err
}

// ListProjectTrailsAll iterates over every Trail that ListProjectTrails
// returns, page by page. It does not report MeProjectTrails.Partial; call
// ListProjectTrails to see it.
func (s *MeService) ListProjectTrailsAll(ctx context.Context, opts *MeProjectTrailListOptions, reqOpts ...RequestOption) iter.Seq2[ProjectTrail, error] {
	o := snapshot(opts)
	return Iterate(func(token string) (*Response[MeProjectTrails], error) {
		p := o
		if token != "" {
			p.PageToken = token
		}
		return s.ListProjectTrails(ctx, &p, reqOpts...)
	}, func(r *Response[MeProjectTrails]) []ProjectTrail { return r.Data.Trails })
}

// meProjectTrail is a Trail as /me/project-trails spells it.
type meProjectTrail struct {
	ID                 string                 `json:"id"`
	Number             int64                  `json:"number"`
	URL                string                 `json:"url"`
	Title              string                 `json:"title"`
	Body               string                 `json:"body"`
	Status             TrailStatus            `json:"status"`
	Phase              TrailPhase             `json:"phase"`
	Type               TrailType              `json:"type"`
	Priority           TrailPriority          `json:"priority"`
	Metadata           map[string]any         `json:"metadata"`
	Assignees          []string               `json:"assignees"`
	AssigneeAccountIDs []string               `json:"assignee_account_ids"`
	AssigneeProfiles   []mePerson             `json:"assignee_profiles"`
	RepoIDs            []string               `json:"repository_ids"`
	AuthorAccountID    string                 `json:"author_account_id"`
	Author             *mePerson              `json:"author"`
	Changes            []meProjectTrailChange `json:"changes"`
	IsPossiblyPartial  bool                   `json:"is_possibly_partial"`
	Capabilities       struct {
		CanEdit        bool `json:"can_edit"`
		CanManageScope bool `json:"can_manage_scope"`
	} `json:"capabilities"`
	Project struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Provider     string `json:"provider"`
		Jurisdiction string `json:"jurisdiction"`
		Reference    struct {
			Forge   string `json:"forge"`
			Project string `json:"project"`
		} `json:"reference"`
		CanManageTrails bool `json:"can_manage_trails"`
	} `json:"project"`
	CreatedAt         Timestamp `json:"created_at"`
	MetadataUpdatedAt Timestamp `json:"metadata_updated_at"`
	LastActivityAt    Timestamp `json:"last_activity_at"`
	LastVisitedAt     Timestamp `json:"last_visited_at"`
	ClosedAt          Timestamp `json:"closed_at"`
}

type mePerson struct {
	AccountID   string   `json:"account_id"`
	Username    string   `json:"username"`
	Login       string   `json:"login"`
	DisplayName string   `json:"display_name"`
	AvatarURL   string   `json:"avatar_url"`
	Identity    Identity `json:"identity"`
}

func (p mePerson) person() TrailPerson { return TrailPerson(p) }

type meProjectTrailChange struct {
	ID             string              `json:"id"`
	RepositoryID   string              `json:"repository_id"`
	Repository     string              `json:"repository"`
	Branch         string              `json:"branch"`
	Number         int64               `json:"number"`
	Title          string              `json:"title"`
	Status         ChangeStatus        `json:"status"`
	HasCodeChanges bool                `json:"has_code_changes"`
	ReadyForReview bool                `json:"ready_for_review"`
	Readiness      string              `json:"readiness"`
	Mergeability   *StoredMergeability `json:"mergeability"`
	CreatedAt      Timestamp           `json:"created_at"`
	UpdatedAt      Timestamp           `json:"updated_at"`
	LastActivityAt Timestamp           `json:"last_activity_at"`
}

func (t meProjectTrail) trail() ProjectTrail {
	out := ProjectTrail{
		ID: t.ID, ProjectID: t.Project.ID, Number: t.Number, URL: t.URL, Title: t.Title, Body: t.Body,
		Status: t.Status, Phase: t.Phase, Type: t.Type, Priority: t.Priority, Metadata: t.Metadata,
		Assignees: t.Assignees, AssigneeAccountIDs: t.AssigneeAccountIDs, RepoIDs: t.RepoIDs,
		AuthorAccountID: t.AuthorAccountID, IsPossiblyPartial: t.IsPossiblyPartial,
		Capabilities: &TrailCapabilities{CanEdit: t.Capabilities.CanEdit, CanManageScope: t.Capabilities.CanManageScope},
		Project: &TrailProject{
			ID: t.Project.ID, Name: t.Project.Name, Provider: t.Project.Provider, Jurisdiction: t.Project.Jurisdiction,
			Host: t.Project.Reference.Forge, PublicName: t.Project.Reference.Project, CanManageTrails: t.Project.CanManageTrails,
		},
		CreatedAt: t.CreatedAt, MetadataUpdatedAt: t.MetadataUpdatedAt, LastActivityAt: t.LastActivityAt,
		LastVisitedAt: t.LastVisitedAt, ClosedAt: t.ClosedAt,
	}
	if t.Author != nil {
		author := t.Author.person()
		out.Author = &author
	}
	for _, p := range t.AssigneeProfiles {
		out.AssigneeProfiles = append(out.AssigneeProfiles, p.person())
	}
	for _, c := range t.Changes {
		out.Changes = append(out.Changes, ProjectTrailChange{
			ID: c.ID, RepositoryID: c.RepositoryID, Repository: c.Repository, Branch: c.Branch, Number: c.Number,
			Title: c.Title, Status: c.Status, HasCodeChanges: c.HasCodeChanges, ReadyForReview: c.ReadyForReview,
			Readiness: c.Readiness, Mergeability: c.Mergeability,
			CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, LastActivityAt: c.LastActivityAt,
		})
	}
	return out
}
