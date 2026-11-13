package entire

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Version is the SDK version reported in the default User-Agent header.
const Version = "0.5.0"

const (
	defaultMaxResponseBytes int64 = 32 << 20
	maxErrorBytes           int64 = 1 << 20
)

// Client is an Entire client. Its services group the API by resource, such
// as Client.Changes and Client.Git, and Client.ResolveRepoID turns a
// repository path into the ID that the repository methods take.
//
// Create a Client with NewClient or NewClientFromEnv. Its configuration is
// immutable, and it is safe for concurrent use when the HTTP transport and
// token source are.
type Client struct {
	transport
	common service

	// Me reads your account, your project Trails across projects, and
	// your sessions across repositories.
	Me *MeService
	// Orgs reads organizations, their members, and their projects.
	Orgs *OrgsService
	// Projects reads projects and their repositories.
	Projects *ProjectsService
	// Clusters lists the clusters that host repositories.
	Clusters *ClustersService
	// Repos reads the repository directory and repositories, ranks the
	// agents that worked on a repository's code, and reads its commits' CI
	// checks.
	Repos *ReposService
	// Git reads a repository's branches, tags, commits, trees, files, and
	// diffs, creates branches, and watches refs change.
	Git *GitService
	// Changes manages a repository's Changes: the Changes themselves,
	// their events, approvals, gates and merges, reviews, and discussions.
	Changes *ChangesService
	// Trails manages project Trails: their intent, their Changes, their
	// timelines, and their discussions.
	Trails *TrailsService
	// Checkpoints reads the agent sessions behind a repository's commits,
	// and their transcripts.
	Checkpoints *CheckpointsService
	// Sessions reads a repository's agent sessions and their transcripts.
	Sessions *SessionsService
	// Search searches code, and Entire's history of repositories,
	// checkpoints, commits, pull requests, and sessions.
	Search *SearchService
	// ServiceAccounts manages service accounts, their grants, OIDC
	// bindings, and secrets, and lists the OIDC providers they can trust.
	ServiceAccounts *ServiceAccountsService
	// Agents manages a project's agents, and reads Entire's catalog of
	// agent models, prices, and presets.
	Agents *AgentsService
	// Environments manages where a project's agents run, and the runners
	// bound to its self-hosted Environments.
	Environments *EnvironmentsService
	// Runs starts, reads, retries, and cancels agent runs.
	Runs *RunsService
	// Runner claims and finishes the runs of self-hosted agents, for a
	// runner's service account.
	Runner *RunnerService
}

// service is what every service holds: the client that sends its requests.
type service struct{ client *Client }

func (c *Client) initServices() {
	c.common.client = c
	c.Me = (*MeService)(&c.common)
	c.Orgs = (*OrgsService)(&c.common)
	c.Projects = (*ProjectsService)(&c.common)
	c.Clusters = (*ClustersService)(&c.common)
	c.Repos = (*ReposService)(&c.common)
	c.Git = (*GitService)(&c.common)
	c.Changes = (*ChangesService)(&c.common)
	c.Trails = (*TrailsService)(&c.common)
	c.Checkpoints = (*CheckpointsService)(&c.common)
	c.Sessions = (*SessionsService)(&c.common)
	c.Search = (*SearchService)(&c.common)
	c.ServiceAccounts = (*ServiceAccountsService)(&c.common)
	c.Agents = (*AgentsService)(&c.common)
	c.Environments = (*EnvironmentsService)(&c.common)
	c.Runs = (*RunsService)(&c.common)
	c.Runner = (*RunnerService)(&c.common)
}

// TokenSource supplies an access token for each request attempt. It can
// refresh OAuth tokens and must be safe for concurrent use when the client is
// shared. An empty token sends no Authorization header.
type TokenSource func(ctx context.Context) (string, error)

// transport is a client's immutable runtime configuration.
type transport struct {
	// baseURL pins every Entire API request to one endpoint. It is nil when
	// routing chooses the cell for each request.
	baseURL *url.URL
	// coreURL pins Core requests to one endpoint. It is nil when the token
	// names the Core.
	coreURL     *url.URL
	routing     *routing
	httpClient  *http.Client
	tokenSource TokenSource
	// rejectToken, when set, drops an access token the API answered with
	// 401, so the next request exchanges for a fresh one. Only the
	// exchangers of a service account secret and of workload identity set
	// it.
	rejectToken      func(token string)
	userAgent        string
	maxResponseBytes int64
	retry            RetryPolicy
	defaults         []RequestOption
}

type clientConfig struct {
	baseURL     string
	coreURL     string
	httpClient  *http.Client
	tokenSource TokenSource
	// customSource is set when WithTokenSource replaced the static token.
	customSource bool
	// tokenFromEnv is set when NewClientFromEnv read the token from
	// ENTIRE_TOKEN, which WithWorkloadIdentity then overrides.
	tokenFromEnv bool
	// workload is set by WithWorkloadIdentity.
	workload         *WorkloadIdentity
	userAgent        string
	maxResponseBytes int64
	retry            RetryPolicy
	defaults         []RequestOption
}

// ClientOption configures a client at construction time.
type ClientOption func(*clientConfig) error

// WithBaseURL sends every Entire API request to baseURL, such as a local
// development server, a test server, or a private deployment, instead of the
// cell that stores the repository or project. Core operations are not
// affected; see WithCoreURL. A bare origin gets /api/v1 appended; a URL with
// a path uses that path as its complete prefix.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *clientConfig) error {
		if baseURL == "" {
			return errors.New("entire: base URL must not be empty")
		}
		c.baseURL = baseURL
		return nil
	}
}

// WithCoreURL sends every Core operation to coreURL, for a local development
// server or a private deployment, instead of the Core that issued the token.
// The client then also asks this Core where repositories and projects live,
// and trusts the API endpoints it reports. A bare origin gets /api/v1
// appended; a URL with a path uses that path as its complete prefix.
func WithCoreURL(coreURL string) ClientOption {
	return func(c *clientConfig) error {
		if coreURL == "" {
			return errors.New("entire: Core URL must not be empty")
		}
		c.coreURL = coreURL
		return nil
	}
}

// WithHTTPClient supplies transport, TLS (including mTLS), proxy, and timeout
// settings. The client is copied; redirects are disabled on the copy so that
// download redirects stay visible and credentials never leave the API origin.
// A Timeout applies to the entire response, including event streams.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *clientConfig) error {
		if client == nil {
			return errors.New("entire: HTTP client must not be nil")
		}
		c.httpClient = client
		return nil
	}
}

// WithTokenSource replaces the static token passed to NewClient.
func WithTokenSource(source TokenSource) ClientOption {
	return func(c *clientConfig) error {
		if source == nil {
			return errors.New("entire: token source must not be nil")
		}
		c.tokenSource = source
		c.customSource = true
		return nil
	}
}

// WithUserAgent replaces the default "entire-sdk-go/<Version>" User-Agent.
func WithUserAgent(value string) ClientOption {
	return func(c *clientConfig) error { c.userAgent = value; return nil }
}

// WithMaxResponseBytes bounds buffered JSON responses. The default is 32 MiB;
// zero removes the limit. Raw responses and event streams are not buffered.
func WithMaxResponseBytes(limit int64) ClientOption {
	return func(c *clientConfig) error {
		if limit < 0 {
			return errors.New("entire: response size limit must not be negative")
		}
		c.maxResponseBytes = limit
		return nil
	}
}

// WithDefaultRequestOptions applies options to every request, before the
// options passed to an individual call. Use it for headers or query
// parameters that every call needs.
func WithDefaultRequestOptions(options ...RequestOption) ClientOption {
	defaults := append([]RequestOption(nil), options...)
	return func(c *clientConfig) error {
		for _, option := range defaults {
			if option == nil {
				return errors.New("entire: nil default request option")
			}
		}
		c.defaults = append(c.defaults, defaults...)
		return nil
	}
}

// NewClient creates a client that sends token as a bearer credential, such
// as the output of `entire auth token`. A service account secret (ent_svc_…)
// is exchanged at Core for short-lived service account access tokens, which
// the client renews as they expire; the secret itself is never sent to the
// API. The exchange uses the Core set by WithCoreURL, or else the Core of
// the jurisdiction the secret names. Every request goes where its data
// lives: Core operations to the Core that issued the token, and Entire API
// operations to the cell that stores the repository or project. Pass an
// empty token for anonymous endpoints, mTLS, or an authenticating transport.
// The default HTTP client has no overall timeout so event streams stay
// usable; use request contexts for deadlines.
func NewClient(token string, options ...ClientOption) (*Client, error) {
	t, err := newTransport(token, options)
	if err != nil {
		return nil, err
	}
	c := &Client{transport: t}
	c.initServices()
	return c, nil
}

// NewClientFromEnv creates a client from the environment:
// ENTIRE_TOKEN supplies the access token (surrounding whitespace is ignored).
// ENTIRE_API_URL and ENTIRE_CORE_URL, when set, work like WithBaseURL and
// WithCoreURL. Options override all three: with WithWorkloadIdentity,
// ENTIRE_TOKEN is ignored.
func NewClientFromEnv(options ...ClientOption) (*Client, error) {
	return NewClient(strings.TrimSpace(os.Getenv("ENTIRE_TOKEN")), withEnv(options)...)
}

func withEnv(options []ClientOption) []ClientOption {
	configured := make([]ClientOption, 0, len(options)+3)
	configured = append(configured, func(c *clientConfig) error {
		c.tokenFromEnv = true
		return nil
	})
	if base := strings.TrimSpace(os.Getenv("ENTIRE_API_URL")); base != "" {
		configured = append(configured, WithBaseURL(base))
	}
	if core := strings.TrimSpace(os.Getenv("ENTIRE_CORE_URL")); core != "" {
		configured = append(configured, WithCoreURL(core))
	}
	return append(configured, options...)
}

func newTransport(token string, options []ClientOption) (transport, error) {
	config := clientConfig{
		httpClient:       &http.Client{},
		tokenSource:      func(context.Context) (string, error) { return token, nil },
		userAgent:        "entire-sdk-go/" + Version,
		maxResponseBytes: defaultMaxResponseBytes,
	}
	for _, option := range options {
		if option == nil {
			return transport{}, errors.New("entire: nil client option")
		}
		if err := option(&config); err != nil {
			return transport{}, err
		}
	}
	if err := config.retry.validate(); err != nil {
		return transport{}, err
	}
	httpClient := *config.httpClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	t := transport{
		httpClient:       &httpClient,
		tokenSource:      config.tokenSource,
		userAgent:        config.userAgent,
		maxResponseBytes: config.maxResponseBytes,
		retry:            config.retry,
		defaults:         config.defaults,
	}
	if config.baseURL != "" {
		base, err := parseBaseURL(config.baseURL, "base URL")
		if err != nil {
			return transport{}, err
		}
		t.baseURL = base
	}
	if config.coreURL != "" {
		core, err := parseBaseURL(config.coreURL, "Core URL")
		if err != nil {
			return transport{}, err
		}
		t.coreURL = core
	}
	t.routing = newRouting()
	if config.workload != nil {
		if (token != "" && !config.tokenFromEnv) || config.customSource {
			return transport{}, errors.New("entire: WithWorkloadIdentity replaces the token; pass an empty token and no WithTokenSource")
		}
		exchanger, err := newWorkloadExchanger(*config.workload, t.coreURL, t.httpClient, t.userAgent)
		if err != nil {
			return transport{}, err
		}
		t.tokenSource = exchanger.tokenSource
		t.rejectToken = exchanger.reject
	} else if !config.customSource && IsServiceAccountSecret(token) {
		// Without WithCoreURL, the secret's jurisdiction names its Core.
		exchanger, err := newSecretExchanger(token, t.coreURL, t.httpClient, t.userAgent)
		if err != nil {
			return transport{}, err
		}
		t.tokenSource = exchanger.tokenSource
		t.rejectToken = exchanger.reject
	}
	return t, nil
}

// parseBaseURL validates an endpoint and gives a bare origin the /api/v1
// prefix.
func parseBaseURL(raw, what string) (*url.URL, error) {
	base, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("entire: invalid %s: %w", what, err)
	}
	if (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" || base.Opaque != "" {
		return nil, fmt.Errorf("entire: %s must be an absolute HTTP(S) URL without credentials, query, or fragment", what)
	}
	if base.Path == "" || base.Path == "/" {
		base.Path, base.RawPath = "/api/v1", ""
	}
	return base, nil
}

// BaseURL returns a copy of the Entire API endpoint set by WithBaseURL or
// ENTIRE_API_URL, including the version prefix, or nil when none is set.
func (c *Client) BaseURL() *url.URL {
	if c.baseURL == nil {
		return nil
	}
	u := *c.baseURL
	return &u
}

// CoreURL returns a copy of the Core endpoint set by WithCoreURL or
// ENTIRE_CORE_URL, including the version prefix, or nil when none is set.
func (c *Client) CoreURL() *url.URL {
	if c.coreURL == nil {
		return nil
	}
	u := *c.coreURL
	return &u
}
