package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	entire "go.entire.com/entire-sdk-go"
)

const (
	// pluginJWTLifetime is the lifetime of a plugin JWT; the guide allows
	// at most five minutes.
	pluginJWTLifetime = 5 * time.Minute
	// pluginJWTBackdate moves a plugin JWT's iat into the past, because Core
	// allows no clock skew: a plugin whose clock runs a little fast would
	// otherwise send a JWT issued in the future.
	pluginJWTBackdate = 30 * time.Second
	// tokenRefreshMargin is how long before expiry a cached token is
	// replaced.
	tokenRefreshMargin = 5 * time.Minute
	// repositoryTTL is how long a repository list is trusted before it is
	// fetched again.
	repositoryTTL = 5 * time.Minute
	// sharedCallTimeout bounds a mint or listing that several callers
	// share, since no single caller's context governs it.
	sharedCallTimeout = 2 * time.Minute
	// maxRepositoryPages bounds a repository listing.
	maxRepositoryPages = 1000
)

// ErrRepositoryNotInstalled reports a repository that is not in the
// installation's repository list: the customer never selected it, or removed
// it.
var ErrRepositoryNotInstalled = errors.New("plugin: repository is not in the installation")

// ErrRepositoryNotReady reports a repository with no processing primary yet,
// which cannot take check runs. List the repositories again later.
var ErrRepositoryNotReady = errors.New("plugin: repository has no processing primary yet")

// Installation acts for one installation of the plugin: it mints its token,
// lists and creates its repositories, writes check runs and deployments, and
// supplies Git credentials.
// Get one with App.Installation. It is safe for concurrent use.
type Installation struct {
	// Checks writes and reads the plugin's check runs on the
	// installation's repositories.
	Checks *Checks
	// Deployments reports the plugin's deployments on the installation's
	// repositories.
	Deployments *Deployments

	app  *App
	id   string
	core *url.URL
	// coreClient sends Core requests: token mints and repository lists.
	coreClient *entire.Client

	mu sync.Mutex
	// tokens holds each token under its request's key.
	tokens     map[string]*tokenSlot
	repos      map[string]Repository
	listedAt   time.Time
	listing    *listCall
	apiClients map[string]*entire.Client
}

// tokenSlot caches one token scope, and shares its mint between callers.
type tokenSlot struct {
	cached *cachedToken
	mint   *mintCall
}

// tokenRequest is the body of a token mint.
type tokenRequest struct {
	RepositoryID string      `json:"repository_id,omitempty"`
	Permissions  Permissions `json:"permissions"`
}

// key names the request's slot: its repository, if any, and permissions.
// Requests without a repository differ by permissions too, so a token
// scoped to repositories:create never stands in for the installation-wide
// one.
func (r tokenRequest) key() string {
	return r.RepositoryID + " " + r.Permissions.String()
}

type cachedToken struct {
	token *Token
	// refresh is when the token is replaced.
	refresh time.Time
}

type mintCall struct {
	done  chan struct{}
	token *Token
	err   error
}

type listCall struct {
	done  chan struct{}
	repos []Repository
	err   error
}

// Installation returns the installation with the given ID, such as a
// Receipt's InstallationID. Installations are global: any Core serves any
// installation. Its calls go to Config.CoreURL when that is set, and
// otherwise to coreURL, such as the Receipt's CoreURL; either must be one of
// Config.Cores. The App keeps one Installation per ID, so its token and
// repository hosts are shared between calls.
func (a *App) Installation(id, coreURL string) (*Installation, error) {
	if id == "" {
		return nil, errors.New("plugin: installation ID is required")
	}
	core, err := a.resolveCore(coreURL)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if inst, ok := a.installations[id]; ok && inst.core.String() == core.String() {
		return inst, nil
	}
	client, err := a.coreClientLocked(core)
	if err != nil {
		return nil, err
	}
	inst := &Installation{
		app:        a,
		id:         id,
		core:       core,
		coreClient: client,
		tokens:     map[string]*tokenSlot{},
		apiClients: map[string]*entire.Client{},
	}
	inst.Checks = &Checks{inst: inst}
	inst.Deployments = &Deployments{inst: inst}
	a.installations[id] = inst
	return inst, nil
}

// ID returns the installation ID.
func (i *Installation) ID() string { return i.id }

// CoreURL returns the Core the installation's calls go to.
func (i *Installation) CoreURL() string { return i.core.String() }

// Token is a short-lived access token for an installation.
//
// Installation.Token returns one that carries every permission registered
// for the plugin, on every repository the customer selected: treat it as
// the installation's full authority. Installation.RepositoryToken returns
// one narrowed to a repository and the permissions requested. Hand only a
// narrowed token to code you trust less than the plugin, such as a
// customer's build.
type Token struct {
	// Value is the token. Send it as a bearer token, or as the password
	// for Git over HTTPS.
	Value string `json:"token"`
	// Username is the Git username to send with the token.
	Username string `json:"username"`
	// ExpiresAt is when the token stops working.
	ExpiresAt time.Time `json:"expires_at"`
	// Permissions are the permissions the token carries.
	Permissions Permissions `json:"permissions"`
}

// Token returns the installation's access token, which carries every
// registered permission on every selected repository. The token is cached
// and replaced five minutes before it expires, and concurrent calls share
// one mint, so callers never need to cache tokens themselves. Entire limits
// mints per installation, and refuses a mint that comes too fast with 403
// Forbidden.
func (i *Installation) Token(ctx context.Context) (*Token, error) {
	return i.token(ctx, tokenRequest{Permissions: i.app.cfg.Permissions})
}

// RepositoryToken returns a token that reaches only the repository with the
// given ID, such as a Repository's ID, with only the permissions requested,
// which must be registered for the plugin. Entire refuses a repository that
// is not in the installation with 404 Not Found. Tokens are cached and
// shared per repository and permissions, as Token's are.
func (i *Installation) RepositoryToken(ctx context.Context, repoID string, permissions Permissions) (*Token, error) {
	if repoID == "" {
		return nil, errors.New("plugin: repository ID is required")
	}
	if permissions.IsZero() {
		return nil, errors.New("plugin: a repository token needs at least one permission")
	}
	return i.token(ctx, tokenRequest{RepositoryID: repoID, Permissions: permissions})
}

func (i *Installation) token(ctx context.Context, req tokenRequest) (*Token, error) {
	key := req.key()
	i.mu.Lock()
	slot := i.tokens[key]
	if slot == nil {
		slot = &tokenSlot{}
		i.tokens[key] = slot
	}
	if slot.cached != nil && i.app.now().Before(slot.cached.refresh) {
		token := slot.cached.token
		i.mu.Unlock()
		return token, nil
	}
	call := slot.mint
	if call == nil {
		call = &mintCall{done: make(chan struct{})}
		slot.mint = call
		// The mint runs apart from its callers, so that one caller's
		// cancellation does not fail the others.
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharedCallTimeout)
			defer cancel()
			minted := i.app.now()
			call.token, call.err = i.mintToken(ctx, req)
			i.mu.Lock()
			slot.mint = nil
			if call.err == nil {
				slot.cached = &cachedToken{call.token, refreshAt(call.token, minted)}
			}
			i.mu.Unlock()
			close(call.done)
		}()
	}
	i.mu.Unlock()
	select {
	case <-call.done:
		return call.token, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// refreshAt is when a token minted at the given time should be replaced:
// five minutes before it expires, or halfway through a shorter life.
func refreshAt(token *Token, minted time.Time) time.Time {
	if life := token.ExpiresAt.Sub(minted); life < 2*tokenRefreshMargin {
		return minted.Add(life / 2)
	}
	return token.ExpiresAt.Add(-tokenRefreshMargin)
}

type pluginClaims struct {
	Issuer         string `json:"iss"`
	Audience       string `json:"aud"`
	PluginID       string `json:"plugin_id"`
	InstallationID string `json:"installation_id,omitempty"`
	IssuedAt       int64  `json:"iat"`
	ExpiresAt      int64  `json:"exp"`
}

// PluginJWT returns a plugin JWT for the installation, addressed to the
// installation's Core, for Core endpoints that the SDK does not cover. It
// lasts five minutes, from 30 seconds ago.
func (i *Installation) PluginJWT() (string, error) {
	return i.app.signPluginJWT(i.core, i.id)
}

// signPluginJWT signs a plugin JWT addressed to core, for the installation
// with the given ID, or for none when it is empty.
func (a *App) signPluginJWT(core *url.URL, installationID string) (string, error) {
	issued := a.now().Add(-pluginJWTBackdate)
	return signJWT(a.cfg.Signer, a.cfg.KeyID, pluginClaims{
		Issuer:         a.cfg.Issuer,
		Audience:       core.JoinPath("api", "authz", "sts").String(),
		PluginID:       a.cfg.Slug,
		InstallationID: installationID,
		IssuedAt:       issued.Unix(),
		ExpiresAt:      issued.Add(pluginJWTLifetime).Unix(),
	})
}

func (i *Installation) mintToken(ctx context.Context, body tokenRequest) (*Token, error) {
	jwt, err := i.PluginJWT()
	if err != nil {
		return nil, err
	}
	r := i.coreClient.NewRequest(http.MethodPost, i.path("access-tokens"), body)
	r.Core = true
	// Minting again is harmless, so a failed mint may be retried.
	r.Idempotent = true
	resp, err := entire.Call[Token](ctx, i.coreClient, r, entire.WithHeader("Authorization", "Bearer "+jwt))
	if err != nil {
		return nil, fmt.Errorf("plugin: mint token for installation %s: %w", i.id, err)
	}
	token := resp.Data
	if token.Value == "" || token.ExpiresAt.IsZero() {
		return nil, fmt.Errorf("plugin: mint token for installation %s: response has no token or expiry", i.id)
	}
	return &token, nil
}

// path returns the Core path of an installation resource.
func (i *Installation) path(resource string) string {
	return "/plugins/" + url.PathEscape(i.app.cfg.Slug) + "/installations/" + url.PathEscape(i.id) + "/" + resource
}

// Repository is a repository the customer selected for the installation.
type Repository struct {
	// ID identifies the repository. It never changes, including on rename
	// or transfer, and is the only field to store.
	ID string `json:"id"`
	// FullName is the repository's name, such as "acme/api". It can change.
	FullName string `json:"full_name"`
	// CloneURL clones the repository with ordinary Git over HTTPS. It can
	// change; see Installation.GitEnv.
	CloneURL string `json:"clone_url"`
	// EntireCloneURL clones the repository through git-remote-entire. It
	// can change; see Installation.RemoteHelperEnv.
	EntireCloneURL string `json:"entire_clone_url"`
	// Placements are the regions that hold the repository.
	Placements []Placement `json:"placements"`
}

// Placement is a region that holds a repository.
type Placement struct {
	// Region names the region, such as "aws-us-east-2".
	Region string `json:"region"`
	// Role is the region's role: RoleProcessingPrimary, RoleMirror, or a
	// role the SDK does not know.
	Role string `json:"role"`
	// APIBaseURL is the region's API host. Only the processing primary has
	// one.
	APIBaseURL string `json:"api_base_url"`
}

// Placement roles.
const (
	// RoleProcessingPrimary is the region that serves check runs.
	RoleProcessingPrimary = "processing_primary"
	// RoleMirror is a region that holds a copy.
	RoleMirror = "mirror"
)

// PrimaryAPIURL returns the API host of the repository's processing
// primary, or an empty string when the repository is not ready for check
// runs yet.
func (r Repository) PrimaryAPIURL() string {
	for _, p := range r.Placements {
		if p.Role == RoleProcessingPrimary {
			return p.APIBaseURL
		}
	}
	return ""
}

// Repositories lists the installation's repositories. The list is the
// authority on what the plugin may touch, and customers change it without
// notice, so this always asks Core. The SDK caches the result to find the
// API host of each repository. A repository whose processing primary is not
// known yet is left out.
func (i *Installation) Repositories(ctx context.Context) ([]Repository, error) {
	i.mu.Lock()
	call, inFlight := i.listing, i.listing != nil
	if !inFlight {
		call = &listCall{done: make(chan struct{})}
		i.listing = call
	}
	i.mu.Unlock()
	if !inFlight {
		// Like a mint, the listing runs apart from its callers.
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharedCallTimeout)
			defer cancel()
			call.repos, call.err = i.list(ctx)
			i.mu.Lock()
			i.listing = nil
			if call.err == nil {
				i.repos = make(map[string]Repository, len(call.repos))
				for _, r := range call.repos {
					i.repos[r.ID] = r
				}
				i.listedAt = i.app.now()
			}
			i.mu.Unlock()
			close(call.done)
		}()
	}
	select {
	case <-call.done:
		if call.err != nil {
			return nil, call.err
		}
		return append([]Repository(nil), call.repos...), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (i *Installation) list(ctx context.Context) ([]Repository, error) {
	var all []Repository
	cursor := ""
	seen := map[string]bool{}
	for page := 0; ; page++ {
		if page == maxRepositoryPages {
			return nil, fmt.Errorf("plugin: repository list of installation %s exceeds %d pages", i.id, maxRepositoryPages)
		}
		// The listing takes the plugin JWT itself, so it costs no mint.
		jwt, err := i.PluginJWT()
		if err != nil {
			return nil, err
		}
		r := i.coreClient.NewRequest(http.MethodGet, i.path("repositories"), nil)
		r.Core = true
		if cursor != "" {
			r.Query.Set("cursor", cursor)
		}
		resp, err := entire.Call[struct {
			Repositories []Repository `json:"repositories"`
			NextCursor   *string      `json:"next_cursor"`
		}](ctx, i.coreClient, r, entire.WithHeader("Authorization", "Bearer "+jwt))
		if err != nil {
			return nil, fmt.Errorf("plugin: list repositories of installation %s: %w", i.id, err)
		}
		all = append(all, resp.Data.Repositories...)
		if resp.Data.NextCursor == nil || *resp.Data.NextCursor == "" {
			return all, nil
		}
		cursor = *resp.Data.NextCursor
		if seen[cursor] {
			return nil, fmt.Errorf("plugin: list repositories of installation %s: %w", i.id, entire.ErrRepeatedPageToken)
		}
		seen[cursor] = true
	}
}

// Repository returns one of the installation's repositories, from the list
// fetched in the last five minutes, or from a new list. It returns
// ErrRepositoryNotInstalled when the repository is not in the installation.
func (i *Installation) Repository(ctx context.Context, id string) (Repository, error) {
	if repo, ok := i.cachedRepository(id); ok {
		return repo, nil
	}
	repos, err := i.Repositories(ctx)
	if err != nil {
		return Repository{}, err
	}
	for _, r := range repos {
		if r.ID == id {
			return r, nil
		}
	}
	return Repository{}, fmt.Errorf("%w: %s", ErrRepositoryNotInstalled, id)
}

func (i *Installation) cachedRepository(id string) (Repository, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	repo, ok := i.repos[id]
	return repo, ok && i.app.now().Sub(i.listedAt) < repositoryTTL
}

// noteAPIURL records a repository's API host named by a verified webhook
// delivery.
func (i *Installation) noteAPIURL(repoID, apiURL string) {
	if repoID == "" || apiURL == "" {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	repo, ok := i.repos[repoID]
	if !ok {
		return
	}
	placements := append([]Placement(nil), repo.Placements...)
	for n := range placements {
		if placements[n].Role == RoleProcessingPrimary {
			placements[n].APIBaseURL = apiURL
			repo.Placements = placements
			i.repos[repoID] = repo
			return
		}
	}
}

// apiClient returns the SDK client for an API version, such as "v1", on an
// API host, which must pass the host policy.
func (i *Installation) apiClient(apiURL, version string) (*entire.Client, error) {
	origin, err := i.app.checkOrigin(apiURL)
	if err != nil {
		return nil, err
	}
	key := origin.JoinPath("api", version).String()
	i.mu.Lock()
	defer i.mu.Unlock()
	if client, ok := i.apiClients[key]; ok {
		return client, nil
	}
	client, err := entire.NewClient("", i.app.clientOptions(entire.WithBaseURL(key))...)
	if err != nil {
		return nil, err
	}
	i.apiClients[key] = client
	return client, nil
}
