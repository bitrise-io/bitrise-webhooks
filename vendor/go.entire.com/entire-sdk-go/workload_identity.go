package entire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.entire.com/entire-sdk-go/internal/corehost"
)

// IDTokenSource returns an OIDC ID token from the platform a workload runs
// on, such as GitHub Actions, issued for audience. The client calls it for
// every token exchange, and passes the audience Core requires: the Core's
// origin followed by /api/authz/sts, such as
// "https://us.auth.entire.io/api/authz/sts". It must be safe for
// concurrent use. Errors must not contain the token.
type IDTokenSource func(ctx context.Context, audience string) (string, error)

// WorkloadIdentity configures WithWorkloadIdentity.
type WorkloadIdentity struct {
	// ServiceAccountID is the service account to sign in as, such as
	// "svc_01JABC...". It needs a binding that trusts the platform; see
	// ServiceAccountsService.CreateBinding. Required.
	ServiceAccountID string
	// IDToken gets the platform's ID tokens. Nil selects one with
	// DetectIDTokenSource when the client is created.
	IDToken IDTokenSource
	// AuthURL is the Core to exchange ID tokens at: the auth URL the
	// binding gives, such as "https://eu.auth.entire.com". It must be a
	// production Core, https://{jurisdiction}.auth.entire.com or the same
	// under entire.io. Empty selects the Core set by WithCoreURL, or else
	// https://us.auth.entire.io. WithCoreURL wins over it.
	AuthURL string
}

// WithWorkloadIdentity signs in as a service account with workload identity
// federation: the client gets an ID token from the platform the code runs
// on, such as GitHub Actions or a Cursor Cloud Agent, and exchanges it at
// Core for a short-lived service account access token. It exchanges again
// shortly before the access token expires, and after the API rejects it
// with 401, so no secret or refresh token is stored. Pass an empty token to
// NewClient; a static token or WithTokenSource is an error.
func WithWorkloadIdentity(identity WorkloadIdentity) ClientOption {
	return func(c *clientConfig) error {
		if !strings.HasPrefix(identity.ServiceAccountID, "svc_") {
			return fmt.Errorf("entire: workload identity needs a service account ID (svc_…), not %q", identity.ServiceAccountID)
		}
		if identity.AuthURL != "" {
			u, err := url.Parse(identity.AuthURL)
			if err != nil || !corehost.IsOrigin(u) {
				return fmt.Errorf("entire: workload identity auth URL %q is not a production Core; use WithCoreURL for another one", identity.AuthURL)
			}
		}
		c.workload = &identity
		return nil
	}
}

// workloadAudiencePath is the path Core appends to its origin for the
// audience of a federated ID token.
const workloadAudiencePath = "/api/authz/sts"

// newWorkloadExchanger exchanges ID tokens from identity at core, or at the
// Core identity names.
func newWorkloadExchanger(identity WorkloadIdentity, core *url.URL, httpClient *http.Client, userAgent string) (*tokenExchanger, error) {
	source := identity.IDToken
	if source == nil {
		detected, err := DetectIDTokenSource()
		if err != nil {
			return nil, err
		}
		source = detected
	}
	if core == nil {
		core = mustParse(defaultCoreURL)
		if identity.AuthURL != "" {
			core = mustParse(identity.AuthURL)
		}
	}
	audience := (&url.URL{Scheme: core.Scheme, Host: core.Host}).String() + workloadAudiencePath
	return newTokenExchanger(identity.ServiceAccountID, core, httpClient, userAgent, func(ctx context.Context) (string, string, error) {
		token, err := source(ctx, audience)
		if err != nil {
			return "", "", fmt.Errorf("entire: get workload ID token: %w", err)
		}
		if token == "" {
			return "", "", errors.New("entire: get workload ID token: the platform returned an empty token")
		}
		return token, tokenTypeJWT, nil
	}), nil
}

// DetectIDTokenSource returns the IDTokenSource of the platform the code runs
// on, from its environment: GitHubActionsIDToken in a GitHub Actions job
// whose workflow grants id-token: write, and CursorIDToken in a Cursor Cloud
// Agent. On other platforms it returns an error; pass a source to
// WithWorkloadIdentity instead, such as BuildkiteIDToken,
// GoogleCloudIDToken, or IDTokenFromFile.
func DetectIDTokenSource() (IDTokenSource, error) {
	switch {
	case os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL") != "":
		return GitHubActionsIDToken(), nil
	case os.Getenv("CURSOR_AGENT_SOCKET") != "":
		return CursorIDToken(), nil
	case os.Getenv("GITHUB_ACTIONS") == "true":
		return nil, errors.New("entire: no workload ID token: this GitHub Actions job lacks ACTIONS_ID_TOKEN_REQUEST_URL; grant the workflow permissions: id-token: write")
	}
	return nil, errors.New("entire: no workload ID token: no supported platform detected; pass an IDTokenSource to WithWorkloadIdentity")
}

// idTokenHTTPClient fetches ID tokens from the platform. Its timeout bounds
// a platform endpoint that never answers when the context has no deadline.
var idTokenHTTPClient = &http.Client{Timeout: 30 * time.Second}

// maxIDTokenBytes bounds an ID token response.
const maxIDTokenBytes = 64 << 10

// GitHubActionsIDToken returns ID tokens from GitHub Actions. It reads
// ACTIONS_ID_TOKEN_REQUEST_URL and ACTIONS_ID_TOKEN_REQUEST_TOKEN, which
// GitHub sets only when the workflow or job grants permissions: id-token:
// write.
func GitHubActionsIDToken() IDTokenSource {
	return func(ctx context.Context, audience string) (string, error) {
		rawURL, bearer := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"), os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
		if rawURL == "" || bearer == "" {
			return "", errors.New("ACTIONS_ID_TOKEN_REQUEST_URL and ACTIONS_ID_TOKEN_REQUEST_TOKEN are not set; grant the workflow permissions: id-token: write")
		}
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return "", errors.New("ACTIONS_ID_TOKEN_REQUEST_URL is not an HTTP(S) URL")
		}
		q := u.Query()
		q.Set("audience", audience)
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return "", fmt.Errorf("request ID token from GitHub Actions: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+bearer)
		req.Header.Set("Accept", "application/json")
		var out struct {
			Value string `json:"value"`
		}
		if err := fetchIDToken(idTokenHTTPClient, req, "GitHub Actions", &out); err != nil {
			return "", err
		}
		return out.Value, nil
	}
}

// CursorIDToken returns ID tokens from a Cursor Cloud Agent, through the
// local socket that CURSOR_AGENT_SOCKET names.
func CursorIDToken() IDTokenSource {
	return func(ctx context.Context, audience string) (string, error) {
		socket := os.Getenv("CURSOR_AGENT_SOCKET")
		if socket == "" {
			return "", errors.New("CURSOR_AGENT_SOCKET is not set; run in a Cursor Cloud Agent")
		}
		client := cursorClient(socket)
		payload, _ := json.Marshal(map[string]string{"aud": audience})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://cursor-agent/v1/tokens/oidc", bytes.NewReader(payload))
		if err != nil {
			return "", fmt.Errorf("request ID token from Cursor: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		var out struct {
			Token string `json:"token"`
		}
		if err := fetchIDToken(client, req, "Cursor", &out); err != nil {
			return "", err
		}
		return out.Token, nil
	}
}

// cursorClients holds one client per Cursor agent socket, so exchanges reuse
// the socket's connection.
var cursorClients sync.Map

// cursorClient returns the client that talks to the Cursor agent at socket.
func cursorClient(socket string) *http.Client {
	if c, ok := cursorClients.Load(socket); ok {
		return c.(*http.Client)
	}
	c, _ := cursorClients.LoadOrStore(socket, &http.Client{
		Timeout: idTokenHTTPClient.Timeout,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socket)
			},
		},
	})
	return c.(*http.Client)
}

// metadataHTTPClient reaches the Google Cloud metadata server directly. Like
// Google's own metadata client, it never uses a proxy: the server is
// link-local, and a proxy would see the ID tokens it returns.
var metadataHTTPClient = &http.Client{
	Timeout:   idTokenHTTPClient.Timeout,
	Transport: &http.Transport{Proxy: nil},
}

// googleMetadataHost is the Google Cloud metadata server. Tests replace it.
var googleMetadataHost = "metadata.google.internal"

// GoogleCloudIDToken returns ID tokens for the Google service account the
// code runs as, from the metadata server of Compute Engine, Cloud Run,
// Cloud Functions, or GKE. The tokens carry the email claim that a
// google-cloud binding pins.
func GoogleCloudIDToken() IDTokenSource {
	return func(ctx context.Context, audience string) (string, error) {
		u := url.URL{Scheme: "http", Host: googleMetadataHost, Path: "/computeMetadata/v1/instance/service-accounts/default/identity", RawQuery: url.Values{"audience": {audience}}.Encode()}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return "", fmt.Errorf("request ID token from Google Cloud: %w", err)
		}
		req.Header.Set("Metadata-Flavor", "Google")
		var token string
		if err := fetchIDToken(metadataHTTPClient, req, "Google Cloud", &token); err != nil {
			return "", err
		}
		return token, nil
	}
}

// BuildkiteIDToken returns ID tokens from a Buildkite job, by running
// `buildkite-agent oidc request-token`. The agent must be on PATH, as it is
// in a job's command.
func BuildkiteIDToken() IDTokenSource {
	return func(ctx context.Context, audience string) (string, error) {
		cmd := exec.CommandContext(ctx, "buildkite-agent", "oidc", "request-token", "--audience", audience)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			// The agent's error output does not hold the token.
			msg := strings.TrimSpace(stderr.String())
			if len(msg) > 512 {
				msg = msg[:512]
			}
			if msg != "" {
				return "", fmt.Errorf("buildkite-agent oidc request-token: %w: %s", err, msg)
			}
			return "", fmt.Errorf("buildkite-agent oidc request-token: %w", err)
		}
		return strings.TrimSpace(string(out)), nil
	}
}

// IDTokenFromFile returns the ID token in the file at path, read again for
// every exchange, for a platform that writes a refreshed token to a file,
// such as a Kubernetes projected service account token. The platform must
// issue it for the audience Core requires, the Core's origin followed by
// /api/authz/sts: the source cannot request one.
func IDTokenFromFile(path string) IDTokenSource {
	return func(context.Context, string) (string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read ID token file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
}

// fetchIDToken sends req and decodes a JSON response into out, or reads a
// plain-text response into out when it is a *string. Errors never contain
// the response body, which may hold a token.
func fetchIDToken(client *http.Client, req *http.Request, platform string, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request ID token from %s: %w", platform, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIDTokenBytes))
	if err != nil {
		return fmt.Errorf("read ID token from %s: %w", platform, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request ID token from %s: HTTP %d", platform, resp.StatusCode)
	}
	if text, ok := out.(*string); ok {
		*text = strings.TrimSpace(string(data))
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode ID token from %s: %w", platform, err)
	}
	return nil
}
