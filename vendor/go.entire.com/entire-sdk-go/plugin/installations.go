package plugin

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"

	entire "go.entire.com/entire-sdk-go"
)

// CodeInstallationSuspended is the entire.HTTPError Code of a call refused
// because the installation is suspended: Core answers 403 to token mints and
// repository lists until the side that suspended it resumes it.
const CodeInstallationSuspended = "installation_suspended"

// LifecycleState is the state of an installation.
type LifecycleState string

// Installation lifecycle states.
const (
	// LifecycleActive is an installation that serves the plugin.
	LifecycleActive LifecycleState = "active"
	// LifecycleSuspended is an installation that the plugin or the customer
	// suspended. It mints no tokens, lists no repositories, and gets no
	// repository events until the same side resumes it.
	LifecycleSuspended LifecycleState = "suspended"
	// LifecycleDeleting is an installation being removed. It serves
	// nothing.
	LifecycleDeleting LifecycleState = "deleting"
)

// SuspendedBy is the side that suspended an installation. Only that side can
// resume it.
type SuspendedBy string

// Sides that suspend installations.
const (
	SuspendedByPlugin   SuspendedBy = "plugin"
	SuspendedByCustomer SuspendedBy = "customer"
)

// Forge is where a project's repositories live.
type Forge string

// Forges.
const (
	ForgeEntire Forge = "entire"
	ForgeGitHub Forge = "github"
)

// Project is the public identity of the project an installation is on.
type Project struct {
	// ID identifies the project. It never changes; key on it.
	ID string `json:"id"`
	// Name is the project's public name, as it appears in Entire's URLs:
	// the project's name on Entire, or the GitHub owner of a GitHub
	// project. It can change.
	Name string `json:"name"`
	// Forge is ForgeEntire or ForgeGitHub.
	Forge Forge `json:"forge"`
}

// InstallationSummary is one of the plugin's installations, as
// App.ListInstallations lists it.
type InstallationSummary struct {
	// ID identifies the installation. Pass it to App.Installation.
	ID string `json:"id"`
	// ProjectID is the project the plugin is installed on.
	ProjectID string `json:"project_id"`
	// LifecycleState is LifecycleActive, LifecycleSuspended, or
	// LifecycleDeleting.
	LifecycleState LifecycleState `json:"lifecycle_state"`
}

// InstallationDetails is an installation as Core reports it after a
// suspension or resumption.
type InstallationDetails struct {
	// ID identifies the installation.
	ID string `json:"installation_id"`
	// ProjectID is the project the plugin is installed on.
	ProjectID string `json:"project_id"`
	// Project is the project's public identity, or nil when Entire cannot
	// resolve it.
	Project *Project `json:"project"`
	// RepositorySelection is "all" or "selected".
	RepositorySelection string `json:"repository_selection"`
	// LifecycleState is the installation's state after the change.
	LifecycleState LifecycleState `json:"lifecycle_state"`
	// SuspendedBy is the side that suspended the installation, while it is
	// suspended.
	SuspendedBy SuspendedBy `json:"suspended_by"`
	// SuspendedAt is when the suspension started, while it is suspended.
	SuspendedAt entire.Timestamp `json:"suspended_at"`
	// CreatedAt is when the customer installed the plugin.
	CreatedAt entire.Timestamp `json:"created_at"`
	// ManageURL is the page where the customer manages the installation,
	// or empty when Entire cannot build it.
	ManageURL string `json:"manage_url"`
	// Permissions are the permissions the installation holds, such as
	// "contents:read".
	Permissions []string `json:"permissions"`
	// PendingPermissions are registered permissions that wait for a
	// project manager to accept them.
	PendingPermissions []string `json:"pending_permissions"`
}

// PluginJWT returns a plugin JWT that names no installation, for Core
// endpoints that act for the plugin as a whole and that the SDK does not
// cover. Like ListInstallations, it is addressed to Config.CoreURL when that
// is set, and otherwise to coreURL, which must be one of Config.Cores, so
// the JWT names the Core the plugin's calls go to. It lasts five minutes,
// from 30 seconds ago. A JWT is valid only at the Core it names.
func (a *App) PluginJWT(coreURL string) (string, error) {
	core, err := a.resolveCore(coreURL)
	if err != nil {
		return "", err
	}
	return a.signPluginJWT(core, "")
}

// ListInstallations returns one page of the plugin's installations, on every
// project and in every lifecycle state, ordered by ID. It is how a plugin
// catches up on installations whose receipt or installation.created event it
// missed, and on suspensions and removals: compare the list with the
// installations you store. An installation no longer listed was removed.
//
// The call takes the plugin's own JWT, so it needs no installation.
// Installations are global, so one Core lists all of them: the call goes to
// Config.CoreURL when that is set, and otherwise to coreURL, which must be
// one of Config.Cores. Pass the listed IDs to App.Installation with the same
// Core, or with a receipt's or delivery's CoreURL, which is nearer the
// project.
//
// opts.PageSize is 1 to 100, and 30 when zero. Response.Next is the
// PageToken of the following page, and empty on the last.
func (a *App) ListInstallations(ctx context.Context, coreURL string, opts *entire.ListOptions) (*entire.Response[[]InstallationSummary], error) {
	core, err := a.resolveCore(coreURL)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	client, err := a.coreClientLocked(core)
	a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	jwt, err := a.signPluginJWT(core, "")
	if err != nil {
		return nil, err
	}
	r := client.NewRequest(http.MethodGet, "/plugins/"+url.PathEscape(a.cfg.Slug)+"/installations", nil)
	r.Core = true
	if opts != nil && opts.PageSize > 0 {
		r.Query.Set("per_page", strconv.Itoa(opts.PageSize))
	}
	if opts != nil && opts.PageToken != "" {
		r.Query.Set("cursor", opts.PageToken)
	}
	type page struct {
		Installations []InstallationSummary `json:"installations"`
		NextCursor    string                `json:"next_cursor"`
	}
	resp, err := entire.Call[page](ctx, client, r, entire.WithHeader("Authorization", "Bearer "+jwt))
	if err != nil {
		return nil, fmt.Errorf("plugin: list installations: %w", err)
	}
	out, _ := unwrap(resp, nil, func(v page) []InstallationSummary { return v.Installations })
	out.Next = resp.Data.NextCursor
	return out, nil
}

// ListInstallationsAll iterates over every one of the plugin's
// installations, fetching pages as it goes, as ListInstallations lists them.
// Each range starts from the first page, or from opts.PageToken.
func (a *App) ListInstallationsAll(ctx context.Context, coreURL string, opts *entire.ListOptions) iter.Seq2[InstallationSummary, error] {
	o := entire.Deref(opts)
	return entire.Iterate(func(next string) (*entire.Response[[]InstallationSummary], error) {
		if next == "" {
			next = o.PageToken
		}
		return a.ListInstallations(ctx, coreURL, &entire.ListOptions{PageSize: o.PageSize, PageToken: next})
	}, func(resp *entire.Response[[]InstallationSummary]) []InstallationSummary { return resp.Data })
}

// Suspend suspends the installation as the plugin: until the plugin resumes
// it, it mints no tokens, lists no repositories, and gets no repository
// events. Suspending again succeeds. Core answers 409 Conflict when the
// customer suspended the installation already, since only that side can
// resume it. Tokens already minted are not revoked.
func (i *Installation) Suspend(ctx context.Context) (*entire.Response[InstallationDetails], error) {
	return i.setSuspended(ctx, http.MethodPut)
}

// Resume resumes an installation that the plugin suspended. Resuming an
// active installation succeeds. Core answers 409 Conflict when the customer
// suspended the installation, since only the customer can resume it then.
func (i *Installation) Resume(ctx context.Context) (*entire.Response[InstallationDetails], error) {
	return i.setSuspended(ctx, http.MethodDelete)
}

func (i *Installation) setSuspended(ctx context.Context, method string) (*entire.Response[InstallationDetails], error) {
	jwt, err := i.PluginJWT()
	if err != nil {
		return nil, err
	}
	r := i.coreClient.NewRequest(method, i.path("suspended"), nil)
	r.Core = true
	// Both directions are idempotent, so a failed attempt may be retried.
	r.Idempotent = true
	resp, err := entire.Call[InstallationDetails](ctx, i.coreClient, r, entire.WithHeader("Authorization", "Bearer "+jwt))
	if err != nil {
		verb := "suspend"
		if method == http.MethodDelete {
			verb = "resume"
		}
		return nil, fmt.Errorf("plugin: %s installation %s: %w", verb, i.id, err)
	}
	return resp, nil
}
