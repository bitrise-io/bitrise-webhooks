package entire

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Route names the repository or project that a request addresses. The SDK's
// methods set it. Set it on a Request you build for Do or Call when the
// endpoint reads or changes a repository or project. The first non-empty
// field is used.
type Route struct {
	// RepoID is a repository ID, such as one from ResolveRepoID.
	RepoID string
	// RepoPath is a repository path, such as "gh/acme/widget" or
	// "et/acme/widget".
	RepoPath string
	// ProjectID is a project ID.
	ProjectID string
	// ProjectPath is a project path, such as "gh/acme" or "et/acme".
	ProjectPath string
}

func (r Route) key() string {
	switch {
	case r.RepoID != "":
		return "repo:" + r.RepoID
	case r.RepoPath != "":
		return "repo-path:" + r.RepoPath
	case r.ProjectID != "":
		return "project:" + r.ProjectID
	case r.ProjectPath != "":
		return "project-path:" + r.ProjectPath
	}
	return ""
}

func (r Route) String() string {
	switch {
	case r.RepoID != "":
		return "repository " + r.RepoID
	case r.RepoPath != "":
		return "repository " + r.RepoPath
	case r.ProjectID != "":
		return "project " + r.ProjectID
	case r.ProjectPath != "":
		return "project " + r.ProjectPath
	}
	return "API endpoint"
}

// Endpoints used when nothing better is known: a token that is not a JWT,
// or an anonymous request with no route. Users never configure these. They
// stay on entire.io until Entire serves its Core and API hosts on
// entire.com.
const (
	defaultCoreURL = "https://us.auth.entire.io/api/v1"
	defaultAPIURL  = "https://aws-us-east-2.api.entire.io/api/v1"
)

// trustedDomains are the domains whose hosts receive credentials without
// the caller naming them. An endpoint taken from a token or from a Core the
// caller did not pin must be an HTTPS host in one of them. entire.io stays
// until Entire's move to entire.com ends.
var trustedDomains = []string{"entire.com", "entire.io", "partial.to"}

const (
	routeTTL       = 10 * time.Minute
	maxRoutes      = 4096
	manifestTTL    = 5 * time.Minute
	maxLookupBytes = 1 << 20
)

// routing caches where routes live and which peers each Core lists in its
// federation manifest. It is safe for concurrent use.
type routing struct {
	mu        sync.Mutex
	routes    map[string]cachedRoute
	manifests map[string]cachedManifest
	now       func() time.Time
}

type cachedRoute struct {
	base    *url.URL
	expires time.Time
}

type cachedManifest struct {
	peers   []string
	expires time.Time
}

func newRouting() *routing {
	return &routing{routes: map[string]cachedRoute{}, manifests: map[string]cachedManifest{}, now: time.Now}
}

// endpoint returns the base URL for req, and whether it came from the route
// cache. A request's own base, set for Core discovery reads, wins; Core
// requests go to Core; Entire API requests go to the pinned endpoint, or to
// the cell that stores what they address.
func (c *Client) endpoint(ctx context.Context, req *Request) (base *url.URL, cached bool, err error) {
	switch {
	case req.base != nil:
		return req.base, false, nil
	case req.Core:
		base, err := c.coreBase(ctx)
		return base, false, err
	case c.baseURL != nil:
		return c.baseURL, false, nil
	}
	return c.locate(ctx, req.Route, true)
}

// coreBase returns the Core to call: the pinned one, or the one named by the
// token's audience or issuer. A token that is not a JWT uses the default
// Core, which redirects requests for other jurisdictions.
func (c *Client) coreBase(ctx context.Context) (*url.URL, error) {
	if c.coreURL != nil {
		return c.coreURL, nil
	}
	token, err := c.tokenSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("entire: obtain access token: %w", err)
	}
	claims, ok := parseClaims(token)
	if !ok {
		return mustParse(defaultCoreURL), nil
	}
	for _, candidate := range append(append([]string(nil), claims.Audience...), claims.Issuer) {
		if origin, err := trustedOrigin(candidate, false); err == nil {
			return origin.JoinPath("api", "v1"), nil
		}
	}
	return nil, fmt.Errorf("entire: the token's issuer %q is not an Entire Core; select one with WithCoreURL", claims.Issuer)
}

// locate returns the cell that serves route: the caller's home cell for the
// zero Route. useCache false forces a fresh lookup.
func (c *Client) locate(ctx context.Context, route Route, useCache bool) (*url.URL, bool, error) {
	key := route.key()
	if key == "" {
		token, err := c.tokenSource(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("entire: obtain access token: %w", err)
		}
		if token == "" {
			return mustParse(defaultAPIURL), false, nil
		}
		claims, _ := parseClaims(token)
		key = "home:" + claims.HomeJurisdiction
		if claims.HomeJurisdiction == "" {
			key = "home-of:" + tokenID(token, claims)
		}
	}
	if useCache {
		if base, ok := c.routing.cached(key); ok {
			return base, true, nil
		}
	}
	base, err := c.resolve(ctx, route)
	if err != nil {
		return nil, false, fmt.Errorf("entire: locate %s: %w", route, err)
	}
	c.routing.store(key, base)
	return base, false, nil
}

func (r *routing) cached(key string) (*url.URL, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.routes[key]
	if !ok || r.now().After(entry.expires) {
		return nil, false
	}
	return entry.base, true
}

func (r *routing) store(key string, base *url.URL) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.routes) >= maxRoutes {
		clear(r.routes)
	}
	r.routes[key] = cachedRoute{base: base, expires: r.now().Add(routeTTL)}
}

// forget drops route from the cache after a cell refused it.
func (r *routing) forget(route Route) {
	if key := route.key(); key != "" {
		r.mu.Lock()
		delete(r.routes, key)
		r.mu.Unlock()
	}
}

// resolve asks Core where route lives. Every answer names a bare origin,
// which must be a trusted host unless the caller pinned the Core.
func (c *Client) resolve(ctx context.Context, route Route) (*url.URL, error) {
	var origin string
	var err error
	switch {
	case strings.Contains(route.RepoID, "/"):
		return nil, fmt.Errorf("%q is a repository path, not an ID; get the ID with ResolveRepoID", route.RepoID)
	case strings.HasPrefix(route.RepoID, "repo_"):
		origin, err = c.publicRepo(ctx, "/v1/repos/"+url.PathEscape(route.RepoID), nil)
	case route.RepoID != "":
		origin, err = c.cellRepo(ctx, route.RepoID)
	case route.RepoPath != "":
		origin, err = c.publicRepo(ctx, "/v1/repos/resolve", url.Values{"path": {route.RepoPath}})
	case route.ProjectID != "":
		var project struct {
			APIURL string `json:"apiUrl"`
		}
		err = c.coreRead(ctx, "/projects/"+url.PathEscape(route.ProjectID), nil, false, &project)
		origin = project.APIURL
	case route.ProjectPath != "":
		host, name, ok := strings.Cut(route.ProjectPath, "/")
		if !ok || host == "" || name == "" || strings.Contains(name, "/") {
			return nil, fmt.Errorf("project path %q is not host/name", route.ProjectPath)
		}
		var resolved struct {
			Project struct {
				APIURL string `json:"apiUrl"`
			} `json:"project"`
		}
		err = c.coreRead(ctx, "/project-resolver/"+url.PathEscape(host)+"/"+url.PathEscape(name), nil, false, &resolved)
		origin = resolved.Project.APIURL
	default:
		origin, err = c.homeCell(ctx)
	}
	if err != nil {
		return nil, err
	}
	if origin == "" {
		return nil, errors.New("its data has no location assigned yet")
	}
	base, err := trustedOrigin(origin, c.coreURL != nil)
	if err != nil {
		return nil, err
	}
	return base.JoinPath("api", "v1"), nil
}

// publicRepo reads Core's public repository resolution, which answers for
// any jurisdiction and names the processing primary's API host.
func (c *Client) publicRepo(ctx context.Context, path string, query url.Values) (string, error) {
	var resolved struct {
		Placements []struct {
			Role       string `json:"role"`
			APIBaseURL string `json:"api_base_url"`
		} `json:"placements"`
	}
	if err := c.coreRead(ctx, path, query, true, &resolved); err != nil {
		return "", err
	}
	for _, p := range resolved.Placements {
		if p.Role == "processing_primary" {
			return p.APIBaseURL, nil
		}
	}
	return "", nil
}

// cellRepo locates a cell's own repository ID. Only that cell knows the ID,
// so the placement it names wins over the processing primary.
func (c *Client) cellRepo(ctx context.Context, id string) (string, error) {
	var repo struct {
		Placements []struct {
			ID         string `json:"id"`
			APIBaseURL string `json:"apiBaseUrl"`
		} `json:"placements"`
		Primaries *struct {
			Processing string `json:"processing"`
		} `json:"primaries"`
	}
	if err := c.coreRead(ctx, "/repos/"+url.PathEscape(id), nil, false, &repo); err != nil {
		return "", err
	}
	want := []string{id}
	if repo.Primaries != nil {
		want = append(want, repo.Primaries.Processing)
	}
	for _, w := range want {
		for _, p := range repo.Placements {
			if p.ID == w && p.APIBaseURL != "" {
				return p.APIBaseURL, nil
			}
		}
	}
	return "", nil
}

// homeCell finds the API host of the caller's home jurisdiction: from the
// token's home_jurisdiction claim, or from Core's /me when the token lacks
// it.
func (c *Client) homeCell(ctx context.Context) (string, error) {
	token, err := c.tokenSource(ctx)
	if err != nil {
		return "", fmt.Errorf("obtain access token: %w", err)
	}
	claims, _ := parseClaims(token)
	home := claims.HomeJurisdiction
	if home == "" {
		var me struct {
			Global struct {
				HomeJurisdiction string `json:"homeJurisdiction"`
			} `json:"global"`
		}
		if err := c.coreRead(ctx, "/me", nil, false, &me); err != nil {
			return "", err
		}
		if home = me.Global.HomeJurisdiction; home == "" {
			return "", errors.New("no home jurisdiction in the token or in Core's /me")
		}
	}
	var listing struct {
		Clusters []struct {
			APIURL       string `json:"apiUrl"`
			IsDefault    bool   `json:"isDefault"`
			Jurisdiction string `json:"jurisdiction"`
		} `json:"clusters"`
	}
	if err := c.coreRead(ctx, "/clusters", nil, false, &listing); err != nil {
		return "", err
	}
	found := ""
	for _, cluster := range listing.Clusters {
		if cluster.Jurisdiction == home && cluster.APIURL != "" && (found == "" || cluster.IsDefault) {
			found = cluster.APIURL
		}
	}
	if found == "" {
		return "", fmt.Errorf("no API host is listed for jurisdiction %q", home)
	}
	return found, nil
}

// coreRead sends a GET to Core for routing and decodes the JSON answer. It
// skips the client's default request options and reads at most 1 MiB.
// public selects Core's /v1 surface instead of /api/v1.
func (c *Client) coreRead(ctx context.Context, path string, query url.Values, public bool, into any) error {
	req := &Request{Method: http.MethodGet, Path: path, Query: query, Header: http.Header{"Accept": {"application/json"}}, Core: true}
	if public {
		base, err := c.coreBase(ctx)
		if err != nil {
			return err
		}
		origin := *base
		origin.Path, origin.RawPath = "", ""
		req.base = &origin
	}
	resp, err := c.send(ctx, req, nil, nil)
	result, err := decode[json.RawMessage](resp, err, maxLookupBytes)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(result.Data, into); err != nil {
		return fmt.Errorf("decode Core response: %w", err)
	}
	return nil
}

// misdirected handles Core's 421, which names the resource's home Core. It
// returns the base URL to replay the request at, or nil when the target is
// not in the responding Core's federation manifest.
func (c *Client) misdirected(ctx context.Context, current *url.URL, apiErr error) *url.URL {
	var httpErr *HTTPError
	if !errors.As(apiErr, &httpErr) {
		return nil
	}
	var body struct {
		HomeCoreURL string `json:"home_core_url"`
	}
	if json.Unmarshal(httpErr.Body, &body) != nil || body.HomeCoreURL == "" {
		return nil
	}
	target, err := url.Parse(body.HomeCoreURL)
	if err != nil || target.Host == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || (target.Scheme != "https" && target.Scheme != current.Scheme) {
		return nil
	}
	if strings.EqualFold(target.Host, current.Host) {
		return nil
	}
	allowed := false
	for _, peer := range c.federation(ctx, current) {
		if p, err := url.Parse(peer); err == nil && p.Scheme == target.Scheme && strings.EqualFold(p.Host, target.Host) {
			allowed = true
		}
	}
	if !allowed {
		return nil
	}
	next := *current
	next.Scheme, next.Host = target.Scheme, target.Host
	return &next
}

// federation returns the sibling Cores that the Core at base lists in its
// public manifest, cached for five minutes. A failed read lists none.
func (c *Client) federation(ctx context.Context, base *url.URL) []string {
	r := c.routing
	origin := url.URL{Scheme: base.Scheme, Host: base.Host}
	key := origin.String()
	r.mu.Lock()
	entry, ok := r.manifests[key]
	r.mu.Unlock()
	if ok && r.now().Before(entry.expires) {
		return entry.peers
	}
	req := &Request{Method: http.MethodGet, Path: "/.well-known/entire-federation", Header: http.Header{"Accept": {"application/json"}}, Anonymous: true, Core: true, base: &origin}
	var manifest struct {
		PeerAuthHosts []string `json:"peer_auth_hosts"`
	}
	resp, err := c.send(ctx, req, nil, nil)
	result, err := decode[json.RawMessage](resp, err, maxLookupBytes)
	if err != nil || json.Unmarshal(result.Data, &manifest) != nil {
		return nil
	}
	r.mu.Lock()
	r.manifests[key] = cachedManifest{peers: manifest.PeerAuthHosts, expires: r.now().Add(manifestTTL)}
	r.mu.Unlock()
	return manifest.PeerAuthHosts
}

// trustedOrigin parses an endpoint that the SDK did not get from the
// caller. It must be a bare origin. Unless trustAny is set, it must also be
// HTTPS on a host in trustedDomains, so a token or a response can never
// direct credentials elsewhere.
func trustedOrigin(raw string, trustAny bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("endpoint %q is not a bare origin", raw)
	}
	if u.Scheme != "https" && (!trustAny || u.Scheme != "http") {
		return nil, fmt.Errorf("endpoint %q does not use HTTPS", raw)
	}
	if !trustAny {
		host := strings.ToLower(u.Hostname())
		trusted := false
		for _, domain := range trustedDomains {
			trusted = trusted || strings.HasSuffix(host, "."+domain)
		}
		if !trusted {
			return nil, fmt.Errorf("endpoint %q is not an Entire host", raw)
		}
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}

// claims are the JWT claims that routing reads. The SDK never verifies the
// signature: the claims only choose where the token goes, and every choice
// is checked by trustedOrigin.
type claims struct {
	Issuer           string
	Audience         []string
	Subject          string
	HomeJurisdiction string
}

func parseClaims(token string) (claims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return claims{}, false
	}
	var raw struct {
		Issuer           string          `json:"iss"`
		Audience         json.RawMessage `json:"aud"`
		Subject          string          `json:"sub"`
		HomeJurisdiction string          `json:"home_jurisdiction"`
	}
	if json.Unmarshal(payload, &raw) != nil {
		return claims{}, false
	}
	c := claims{Issuer: raw.Issuer, Subject: raw.Subject, HomeJurisdiction: raw.HomeJurisdiction}
	var one string
	if json.Unmarshal(raw.Audience, &one) == nil && one != "" {
		c.Audience = []string{one}
	} else {
		_ = json.Unmarshal(raw.Audience, &c.Audience)
	}
	return c, true
}

// tokenID keys a caller's home cell without keeping the token itself.
func tokenID(token string, c claims) string {
	if c.Subject != "" {
		return c.Issuer + "|" + c.Subject
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:16])
}

func mustParse(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
