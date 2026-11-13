package entire

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"strings"
)

// ProjectsService reads projects and their repositories.
type ProjectsService service

// Project is a project: the repositories of one GitHub owner, or a group of
// repositories hosted by Entire, owned by an organization or an account.
type Project struct {
	// ID identifies the project; pass it to ProjectsService methods as
	// projectID.
	ID string `json:"id"`
	// Name is the project's name: the GitHub owner for a GitHub project, such
	// as "acme". Resolve matches it regardless of case.
	Name string `json:"name"`
	// OwnerType is the kind of owner: "org" for an organization, or
	// "account".
	OwnerType string `json:"ownerType"`
	// OwnerID identifies the owner, the organization or account that
	// OwnerType names.
	OwnerID string `json:"ownerId"`
	// OwnerName is the owner's name.
	OwnerName string `json:"ownerName"`
	// Region is where the project's data lives, such as "us".
	Region string `json:"region"`
	// PrimaryProcessingCell is the ID of the cell that processes the
	// project's repositories and holds its trails, such as
	// "aws-us-east-2". It is not a cluster slug, and it is empty while the
	// project has no cell assigned.
	PrimaryProcessingCell string `json:"primaryProcessingCell"`
	// APIURL is the base URL of the Entire API of PrimaryProcessingCell.
	// The SDK sends a project's requests there on its own. It is empty when
	// the project has no cell assigned or Core cannot route to it; don't
	// fall back to another cell.
	APIURL string `json:"apiUrl"`
	// Plugins are the plugins installed on the project, ordered by slug.
	// Get and Resolve set it, and only when the caller may read or manage
	// the project; it is nil in listings, and empty when no plugin is
	// installed.
	Plugins []ProjectPlugin `json:"plugins"`
	// CreatedAt is when the project was created.
	CreatedAt Timestamp `json:"createdAt"`
	// RepositoryCount counts the project's published repositories. Only
	// paginated listings set it, and only when the caller may see the whole
	// project; it is zero otherwise.
	RepositoryCount int64 `json:"repositoryCount"`
	// Capabilities says what the caller may do with the project.
	Capabilities ProjectCapabilities `json:"capabilities"`
}

// ProjectCapabilities says what the caller may do with a project.
type ProjectCapabilities struct {
	// CanCreateRepository reports whether the caller may create
	// repositories in the project.
	CanCreateRepository bool `json:"canCreateRepository"`
	// CanManageTrails reports whether the caller may manage the project's
	// trails.
	CanManageTrails bool `json:"canManageTrails"`
	// CanManageAccess reports whether the caller may manage who can access
	// the project.
	CanManageAccess bool `json:"canManageAccess"`
	// CanDelete reports whether the caller may delete the project.
	CanDelete bool `json:"canDelete"`
	// CanManagePlugins reports whether the caller may install, manage, and
	// uninstall the project's plugins: a project manager, or an owner of
	// the GitHub organization behind a GitHub project.
	CanManagePlugins bool `json:"canManagePlugins"`
	// CanManageAgents reports whether the caller may view and manage the
	// project's saved agents, environments, and automations. It is false
	// against a Core that predates it.
	CanManageAgents bool `json:"canManageAgents"`
}

// ProjectPlugin is a plugin installed on a project.
type ProjectPlugin struct {
	// Slug names the plugin.
	Slug string `json:"slug"`
	// RepositorySelection is "all" when the installation covers every
	// repository of the project, or "selected" when it covers some of them.
	RepositorySelection string `json:"repository_selection"`
}

// ResolvedProject is a project that ProjectsService.Resolve found by path,
// with the path's canonical spelling.
type ResolvedProject struct {
	Project
	// Reference is the project's canonical path, which can differ from the
	// path given to Resolve in case.
	Reference ProjectReference
}

// ProjectReference is a project's canonical path, independent of the name
// that stores it.
type ProjectReference struct {
	// ForgePrefix is "gh" for a GitHub owner's project or "et" for a
	// project hosted by Entire.
	ForgePrefix string `json:"forge_prefix"`
	// Project is the project's name, in its canonical case.
	Project string `json:"project"`
}

// String returns the reference as a project path, such as "gh/acme".
func (r ProjectReference) String() string { return r.ForgePrefix + "/" + r.Project }

// projectPage is the envelope of the project listings.
type projectPage struct {
	Projects      []Project `json:"projects"`
	NextPageToken string    `json:"nextPageToken"`
}

// List returns one page of the projects that the caller can access.
func (s *ProjectsService) List(ctx context.Context, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]Project], error) {
	req := coreRequest(http.MethodGet, nil, "/projects")
	Deref(opts).add(req.Query, "pageSize", "pageToken")
	return list(ctx, s.client, req, reqOpts, func(e *projectPage) page[Project] {
		return page[Project]{items: e.Projects, next: e.NextPageToken}
	})
}

// ListAll iterates over every project that List returns, page by page.
func (s *ProjectsService) ListAll(ctx context.Context, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[Project, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Project], error) {
		return s.List(ctx, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// Get returns a project.
func (s *ProjectsService) Get(ctx context.Context, projectID string, reqOpts ...RequestOption) (*Response[Project], error) {
	return Call[Project](ctx, s.client, coreRequest(http.MethodGet, nil, "/projects/%s", projectID), reqOpts...)
}

// Resolve returns the project at projectPath: "gh/owner" for a GitHub
// owner's project, or "et/name" for a project hosted by Entire, with the
// path's canonical spelling. Names match regardless of case. A project the
// caller cannot see fails with 404 Not Found, like one that does not exist.
func (s *ProjectsService) Resolve(ctx context.Context, projectPath string, reqOpts ...RequestOption) (*Response[ResolvedProject], error) {
	host, name, ok := strings.Cut(strings.TrimPrefix(projectPath, "/"), "/")
	req := coreRequest(http.MethodGet, nil, "/project-resolver/%s/%s", host, name)
	if !ok || (host != "gh" && host != "et") || name == "" || strings.Contains(name, "/") {
		req.invalid = fmt.Errorf("entire: project path %q is not gh/name or et/name", projectPath)
	}
	type envelope struct {
		Project   Project `json:"project"`
		Reference struct {
			ProjectReference
			Host string `json:"host"` // deprecated spelling of forge_prefix
		} `json:"reference"`
	}
	return unwrap(ctx, s.client, req, reqOpts, func(e *envelope) ResolvedProject {
		ref := e.Reference.ProjectReference
		if ref.ForgePrefix == "" {
			ref.ForgePrefix = e.Reference.Host
		}
		return ResolvedProject{Project: e.Project, Reference: ref}
	})
}

// ListRepos returns one page of a project's repositories.
func (s *ProjectsService) ListRepos(ctx context.Context, projectID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]Repo], error) {
	req := coreRequest(http.MethodGet, nil, "/projects/%s/repos", projectID)
	Deref(opts).add(req.Query, "pageSize", "pageToken")
	type envelope struct {
		Repos         []Repo `json:"repos"`
		NextPageToken string `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[Repo] {
		return page[Repo]{items: e.Repos, next: e.NextPageToken}
	})
}

// ListReposAll iterates over every repository that ListRepos returns, page by
// page.
func (s *ProjectsService) ListReposAll(ctx context.Context, projectID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[Repo, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]Repo], error) {
		return s.ListRepos(ctx, projectID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}
