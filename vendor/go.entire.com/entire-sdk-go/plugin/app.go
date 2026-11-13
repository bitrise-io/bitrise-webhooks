package plugin

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	entire "go.entire.com/entire-sdk-go"
	"go.entire.com/entire-sdk-go/internal/corehost"
)

// Access is a level of access to one kind of resource.
type Access string

// Access levels.
const (
	Read  Access = "read"
	Write Access = "write"
	// Create is the only level of Permissions.Repositories.
	Create Access = "create"
)

// Permissions are the permissions a plugin registers, or a token requests. A
// zero field grants nothing.
type Permissions struct {
	// Contents: Read clones the repository.
	Contents Access `json:"contents,omitempty"`
	// Checks: Write writes and reads the plugin's check runs.
	Checks Access `json:"checks,omitempty"`
	// Discussions: Write starts discussions on a repository's trails,
	// replies to them, and edits and deletes the installation's own
	// messages. It reads nothing: listing discussions, and
	// Installation.RepositoryClient, also need Contents: Read. A plugin
	// that reports review findings on code registers Findings instead.
	Discussions Access `json:"discussions,omitempty"`
	// Findings: Read reads the review findings on the repositories'
	// trails. Write also starts reviews, publishes findings in them, and
	// updates the findings of the reviews the installation started.
	// Neither reads repository contents or trails, nor gates a merge:
	// Installation.RepositoryClient also needs Contents: Read.
	Findings Access `json:"findings,omitempty"`
	// Deployments: Write reports and reads the plugin's deployments.
	// Each project must accept it before it takes effect there.
	Deployments Access `json:"deployments,omitempty"`
	// Installations: Read reads the installation itself. Every project
	// installation has it, so a token carries it whether or not the plugin
	// registers it.
	Installations Access `json:"installations,omitempty"`
	// Repositories: Create creates repositories in the installation's
	// project, through Installation.CreateRepository. It reaches no
	// repository, not even one it created: register Contents too to read
	// or push to the new repository.
	Repositories Access `json:"repositories,omitempty"`
	// Trails: Read reads the repositories' trails, and Write also edits
	// them. Neither reads nor writes repository contents.
	Trails Access `json:"trails,omitempty"`
}

// IsZero reports whether p grants nothing.
func (p Permissions) IsZero() bool { return p == Permissions{} }

func (p Permissions) String() string {
	var parts []string
	for _, f := range []struct {
		name   string
		access Access
	}{{"checks", p.Checks}, {"contents", p.Contents}, {"deployments", p.Deployments}, {"discussions", p.Discussions}, {"findings", p.Findings}, {"installations", p.Installations}, {"repositories", p.Repositories}, {"trails", p.Trails}} {
		if f.access != "" {
			parts = append(parts, f.name+":"+string(f.access))
		}
	}
	return strings.Join(parts, ",")
}

// Config is a plugin's registration with Entire, and how the SDK runs it.
type Config struct {
	// Slug is the plugin slug Entire assigned, such as "acme-plugin": 3 to
	// 40 lowercase letters and digits, in words joined by single hyphens.
	Slug string
	// Issuer is the iss of the plugin JWTs, as registered.
	Issuer string
	// Signer signs plugin JWTs. It must hold an Ed25519 key: an
	// ed25519.PrivateKey, or a KMS or HSM key that implements crypto.Signer.
	Signer crypto.Signer
	// KeyID is the kid of Signer's key in the plugin's JWKS. A plugin
	// registered with a JWKS URL requires it. A plugin registered with
	// public keys may leave it empty: Entire tries each registered key.
	KeyID string
	// PublishedKeys are further public keys that JWKSHandler serves beside
	// Signer's, such as the next key during a rotation.
	PublishedKeys []PublicKey
	// Permissions are the permissions registered for the plugin; at least
	// one is required. Installation.Token requests them, and Entire issues
	// a token for every repository only when they name every registered
	// permission: otherwise it answers 400 with code repository_required.
	Permissions Permissions
	// Audience is the aud registered for install receipts, usually the
	// plugin's base URL. AcceptReceipt requires it.
	Audience string

	// DeliveryJWKSURL is the JWKS URL Entire gave for delivery JWTs.
	// Webhooks require it. It is trusted as configured.
	DeliveryJWKSURL string
	// DeliveryAudience is the aud of delivery JWTs. Empty selects the
	// plugin slug, which Entire uses today.
	DeliveryAudience string
	// DeliveryIssuer, when set, is the only iss accepted on delivery JWTs.
	// Each Entire cell signs deliveries as its own public origin, so empty
	// accepts any https origin that HostPolicy approves, and the web app's
	// origin, "https://entire.com" or "https://entire.io", which a cell
	// without a public origin uses. Either
	// way the keys come from DeliveryJWKSURL, which every cell shares.
	DeliveryIssuer string

	// Cores are the Cores the plugin trusts: install receipts must come from
	// one of them, and installation calls go to one of them. Empty trusts
	// every production Core, including those Entire opens after this SDK's
	// release: see IsEntireCore. Set it for a non-production deployment.
	Cores []string
	// CoreURL, when set, is the Core every installation call goes to, such
	// as the one in the region where the plugin runs. Installations are
	// global, so any Core serves any installation. Empty uses the Core
	// passed to App.Installation, usually the one that issued the receipt.
	CoreURL string

	// EntireURL is where customers install plugins. Empty selects
	// "https://entire.com".
	EntireURL string
	// HTTPClient sends every request: API calls, token mints, and JWKS
	// fetches. Nil selects a client with a 30-second timeout.
	HTTPClient *http.Client
	// ClientOptions configure the SDK clients that call Entire, after the
	// plugin's defaults: a retry policy of four attempts on 408, 429, 502,
	// 503 (including no_primary), and 504.
	ClientOptions []entire.ClientOption
	// ReplayStore rejects reused receipt and delivery JWTs. Nil selects a
	// MemoryStore, which protects a single process only.
	ReplayStore ReplayStore
	// DeliveryStore skips webhook deliveries already handled. Nil selects
	// a MemoryStore, which protects a single process only.
	DeliveryStore DeliveryStore
	// Logger records webhook failures that the handler answers with 500.
	// Nil selects slog.Default().
	Logger *slog.Logger
	// HostPolicy approves every URL that the SDK takes from Entire before
	// sending a credential to it: API hosts, clone URLs, and the API hosts in
	// webhook deliveries. Nil selects the policy in the plugin guide: an
	// https or entire URL on a host under entire.com, or under entire.io
	// while Entire moves there. Replace it only to test against a local
	// server. Cores are checked against Cores instead.
	HostPolicy func(u *url.URL) error
}

// DefaultCores are Entire's production Cores when this SDK was released.
// A plugin that leaves Config.Cores empty trusts these and any Core Entire
// opens later, by IsEntireCore. Entire is moving its hosts from entire.io to
// entire.com, so each Core is listed under both names until the move ends.
var DefaultCores = []string{
	"https://us.auth.entire.com",
	"https://eu.auth.entire.com",
	"https://au.auth.entire.com",
	"https://in.auth.entire.com",
	"https://uk.auth.entire.com",
	"https://sg.auth.entire.com",
	"https://br.auth.entire.com",
	"https://us.auth.entire.io",
	"https://eu.auth.entire.io",
	"https://au.auth.entire.io",
	"https://in.auth.entire.io",
	"https://uk.auth.entire.io",
	"https://sg.auth.entire.io",
	"https://br.auth.entire.io",
}

// IsEntireCore reports whether u is the origin of a production Core:
// https://{jurisdiction}.auth.entire.com, or the same under entire.io while
// Entire moves to entire.com. Entire names every Core this way and keeps
// auth.entire.com and auth.entire.io for Cores alone, so a new jurisdiction
// needs no SDK release.
func IsEntireCore(u *url.URL) bool {
	return corehost.IsOrigin(u)
}

// entireDomains are the domains Entire's hosts live under: entire.com, and
// entire.io until Entire's move to entire.com ends.
var entireDomains = []string{"entire.com", "entire.io"}

// DefaultHostPolicy accepts https and entire URLs on hosts under entire.com
// or entire.io, without credentials. It is the HostPolicy when Config sets
// none.
func DefaultHostPolicy(u *url.URL) error {
	if u.Scheme != "https" && u.Scheme != "entire" {
		return fmt.Errorf("plugin: %s does not use https", u.Redacted())
	}
	if u.User != nil {
		return fmt.Errorf("plugin: %s carries credentials", u.Redacted())
	}
	host := strings.ToLower(u.Hostname())
	for _, domain := range entireDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return nil
		}
	}
	return fmt.Errorf("plugin: %s is not an Entire host", u.Redacted())
}

// App is a registered plugin. Create one with New and share it: it caches
// signing keys, tokens, and repository hosts, and is safe for concurrent
// use.
type App struct {
	cfg        Config
	publicKey  ed25519.PublicKey
	httpClient *http.Client
	// cores are the trusted Core origins. With none configured, the
	// production Cores are trusted by IsEntireCore instead.
	cores  map[string]bool
	logger *slog.Logger
	now    func() time.Time

	mu            sync.Mutex
	receiptKeys   map[string]*keySet
	deliveryKeys  *keySet
	installations map[string]*Installation
	// coreClients holds an SDK client per Core origin.
	coreClients map[string]*entire.Client
}

// New validates cfg and returns the plugin.
func New(cfg Config) (*App, error) {
	switch {
	case cfg.Slug == "":
		return nil, errors.New("plugin: Config.Slug is required")
	case !validSlug(cfg.Slug):
		return nil, fmt.Errorf("plugin: Config.Slug %q is not a plugin slug: 3 to 40 lowercase letters and digits, with single hyphens between them", cfg.Slug)
	case cfg.Issuer == "":
		return nil, errors.New("plugin: Config.Issuer is required")
	case cfg.Signer == nil:
		return nil, errors.New("plugin: Config.Signer is required")
	case cfg.KeyID == "" && len(cfg.PublishedKeys) > 0:
		return nil, errors.New("plugin: Config.KeyID is required with PublishedKeys")
	}
	public, ok := cfg.Signer.Public().(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("plugin: Config.Signer holds a %T, not an Ed25519 key", cfg.Signer.Public())
	}
	for _, k := range cfg.PublishedKeys {
		if k.KeyID == "" || len(k.Key) != ed25519.PublicKeySize {
			return nil, errors.New("plugin: every published key needs a KeyID and an Ed25519 key")
		}
		if k.KeyID == cfg.KeyID {
			return nil, fmt.Errorf("plugin: published key %q repeats Config.KeyID", k.KeyID)
		}
	}
	if cfg.Permissions.IsZero() {
		return nil, errors.New("plugin: Config.Permissions must name at least one registered permission")
	}
	if cfg.DeliveryAudience == "" {
		cfg.DeliveryAudience = cfg.Slug
	}
	cores := make(map[string]bool, len(cfg.Cores))
	for _, raw := range cfg.Cores {
		core, err := parseOrigin(raw)
		if err != nil {
			return nil, fmt.Errorf("plugin: Config.Cores: %w", err)
		}
		cores[core.String()] = true
	}
	if cfg.CoreURL != "" {
		core, err := parseOrigin(cfg.CoreURL)
		if err != nil || !trustsCore(cores, core) {
			return nil, fmt.Errorf("plugin: Config.CoreURL %q is not one of Config.Cores", cfg.CoreURL)
		}
	}
	if cfg.EntireURL == "" {
		cfg.EntireURL = "https://entire.com"
	}
	if cfg.HostPolicy == nil {
		cfg.HostPolicy = DefaultHostPolicy
	}
	store := NewMemoryStore()
	if cfg.ReplayStore == nil {
		cfg.ReplayStore = store
	}
	if cfg.DeliveryStore == nil {
		cfg.DeliveryStore = store
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	if cfg.HTTPClient != nil {
		copied := *cfg.HTTPClient
		httpClient = &copied
	}
	// Credentials and keys come only from the URLs Entire names.
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	cfg.PublishedKeys = append([]PublicKey(nil), cfg.PublishedKeys...)
	cfg.ClientOptions = append([]entire.ClientOption(nil), cfg.ClientOptions...)
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		cfg:           cfg,
		cores:         cores,
		logger:        logger,
		publicKey:     public,
		httpClient:    httpClient,
		now:           time.Now,
		receiptKeys:   map[string]*keySet{},
		installations: map[string]*Installation{},
		coreClients:   map[string]*entire.Client{},
	}, nil
}

// Slug bounds, as Entire enforces them on registration.
const (
	minSlugLen = 3
	maxSlugLen = 40
)

// slugPattern is Entire's rule for a plugin slug: lowercase letters and
// digits, in words joined by single hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// validSlug reports whether slug is one Entire can register.
func validSlug(slug string) bool {
	return len(slug) >= minSlugLen && len(slug) <= maxSlugLen && slugPattern.MatchString(slug)
}

// Slug returns the plugin slug.
func (a *App) Slug() string { return a.cfg.Slug }

// JWKSHandler serves the plugin's public keys as a JWKS, for the JWKS URL
// registered with Entire: Signer's key, then PublishedKeys. It needs
// Config.KeyID, and answers 500 without one.
func (a *App) JWKSHandler() http.Handler {
	keys := append([]PublicKey{{KeyID: a.cfg.KeyID, Key: a.publicKey}}, a.cfg.PublishedKeys...)
	doc, err := marshalJWKS(keys)
	if a.cfg.KeyID == "" {
		err = errors.New("plugin: Config.KeyID is required to serve a JWKS")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(doc)
	})
}

// NewState returns an unguessable value for an install's state parameter.
// Tie it to the customer's session, and pass it to AcceptReceipt when they
// return.
func NewState() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// InstallURL returns the page to send a customer's browser to, to install
// the plugin. redirectURI must exactly match a registered callback URL, and
// state must be 1 to 1,024 characters; NewState makes one.
func (a *App) InstallURL(redirectURI, state string) (string, error) {
	if redirectURI == "" {
		return "", errors.New("plugin: redirect URI is required")
	}
	if n := len(state); n < 1 || n > 1024 {
		return "", errors.New("plugin: state must be 1 to 1024 characters")
	}
	base, err := url.Parse(a.cfg.EntireURL)
	if err != nil {
		return "", fmt.Errorf("plugin: invalid Config.EntireURL: %w", err)
	}
	u := base.JoinPath("plugins", a.cfg.Slug, "install")
	u.RawQuery = url.Values{"redirect_uri": {redirectURI}, "state": {state}}.Encode()
	return u.String(), nil
}

// checkURL parses raw and applies the host policy. scheme, when set, is the
// only scheme accepted.
func (a *App) checkURL(raw, scheme string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("plugin: %q is not an absolute URL", raw)
	}
	if scheme != "" && u.Scheme != scheme {
		return nil, fmt.Errorf("plugin: %s does not use %s", u.Redacted(), scheme)
	}
	if err := a.cfg.HostPolicy(u); err != nil {
		return nil, err
	}
	return u, nil
}

// checkOrigin is checkURL for a URL that must be a bare origin, such as a
// Core or an API host. It returns the origin.
func (a *App) checkOrigin(raw string) (*url.URL, error) {
	u, err := a.checkURL(raw, "")
	if err != nil {
		return nil, err
	}
	if u.Scheme == "entire" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("plugin: %s is not a bare origin", u.Redacted())
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}

// parseOrigin parses a bare origin, such as a Core.
func parseOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("%q is not a bare http(s) origin", raw)
	}
	return &url.URL{Scheme: u.Scheme, Host: strings.ToLower(u.Host)}, nil
}

// checkCore returns raw's origin if it is one of the trusted Cores.
func (a *App) checkCore(raw string) (*url.URL, error) {
	core, err := parseOrigin(raw)
	if err != nil {
		return nil, fmt.Errorf("plugin: Core %w", err)
	}
	if !trustsCore(a.cores, core) {
		return nil, fmt.Errorf("plugin: %s is not a trusted Core; see Config.Cores", core)
	}
	return core, nil
}

// resolveCore returns the Core a call goes to: Config.CoreURL when it is set,
// and otherwise coreURL, which must be a trusted Core.
func (a *App) resolveCore(coreURL string) (*url.URL, error) {
	if a.cfg.CoreURL != "" {
		coreURL = a.cfg.CoreURL
	}
	if coreURL == "" {
		return nil, errors.New("plugin: a Core is required; set Config.CoreURL or pass the receipt's CoreURL")
	}
	return a.checkCore(coreURL)
}

// coreClientLocked returns the SDK client for a Core. a.mu must be held.
func (a *App) coreClientLocked(core *url.URL) (*entire.Client, error) {
	if client, ok := a.coreClients[core.String()]; ok {
		return client, nil
	}
	client, err := entire.NewClient("", a.clientOptions(entire.WithCoreURL(core.String()))...)
	if err != nil {
		return nil, err
	}
	a.coreClients[core.String()] = client
	return client, nil
}

// trustsCore reports whether core is one of the configured Cores, or, with
// none configured, a production Core.
func trustsCore(cores map[string]bool, core *url.URL) bool {
	if len(cores) == 0 {
		return IsEntireCore(core)
	}
	return cores[core.String()]
}

// clientOptions are the options of every SDK client the plugin creates.
func (a *App) clientOptions(extra ...entire.ClientOption) []entire.ClientOption {
	options := []entire.ClientOption{
		entire.WithHTTPClient(a.httpClient),
		// Four attempts with identical bodies. Check-run writes are
		// idempotent, so a retry after a 503 that followed a committed
		// write is safe.
		entire.WithRetryPolicy(entire.RetryPolicy{MaxAttempts: 4}),
		entire.WithUserAgent("entire-sdk-go/" + entire.Version + " plugin/" + a.cfg.Slug),
	}
	options = append(options, a.cfg.ClientOptions...)
	return append(options, extra...)
}

func (a *App) receiptKeySet(jwksURL string) *keySet {
	a.mu.Lock()
	defer a.mu.Unlock()
	set, ok := a.receiptKeys[jwksURL]
	if !ok {
		set = &keySet{url: jwksURL, http: a.httpClient, now: a.now}
		a.receiptKeys[jwksURL] = set
	}
	return set
}
