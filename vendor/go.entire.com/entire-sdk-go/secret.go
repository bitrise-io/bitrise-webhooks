package entire

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.entire.com/entire-sdk-go/internal/corehost"
)

// ServiceAccountSecretPrefix starts every service account secret. A token
// with this prefix is not sent to the API: NewClient exchanges it for a
// short-lived service account access token first.
const ServiceAccountSecretPrefix = "ent_svc_"

const (
	// secretClientID is the public OAuth client the exchange names.
	secretClientID = "entire-cli"
	// secretRefreshMargin renews the access token this long before it
	// expires, so a request never carries a token that lapses in flight. A
	// token that lives less than twice as long renews at half its lifetime.
	secretRefreshMargin = time.Minute
	// secretFallbackTTL applies when Core reports no lifetime.
	secretFallbackTTL = 5 * time.Minute
)

// IsServiceAccountSecret reports whether token is a service account secret.
func IsServiceAccountSecret(token string) bool {
	return strings.HasPrefix(token, ServiceAccountSecretPrefix)
}

// errMalformedSecret is the refusal for a secret that fails the offline
// checks. It never names the secret.
var errMalformedSecret = errors.New("entire: malformed service account secret")

const (
	// secretRandomLen is the length of 32 random bytes in unpadded base32.
	secretRandomLen = 52
	// secretCheckLen is the length of the base62 CRC-32 that ends a secret.
	secretCheckLen = 6
	ulidLen        = 26
	base32Lower    = "abcdefghijklmnopqrstuvwxyz234567"
	base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	crockford      = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

// secretBase32 encodes <random>: lowercase RFC 4648 base32 without padding.
var secretBase32 = base32.NewEncoding(base32Lower).WithPadding(base32.NoPadding)

// parsedSecret is what a service account secret names.
type parsedSecret struct {
	// jurisdiction is the jurisdiction of the account's project, such as
	// "us". It names the Core that exchanges the secret.
	jurisdiction string
	// accountID is the service account ID, with its svc_ prefix.
	accountID string
}

// parseServiceAccountSecret validates a v1 secret offline:
// ent_svc_<jurisdiction>_<account ULID>_<secret ULID>_<random><check>, where
// <random> is 52 characters of lowercase base32 and <check> is the CRC-32
// (IEEE) of everything before it, in 6 base62 digits. Errors never contain
// the secret.
func parseServiceAccountSecret(secret string) (parsedSecret, error) {
	rest, ok := strings.CutPrefix(secret, ServiceAccountSecretPrefix)
	if !ok {
		return parsedSecret{}, errMalformedSecret
	}
	parts := strings.Split(rest, "_")
	if len(parts) != 4 {
		return parsedSecret{}, fmt.Errorf("%w: want ent_svc_<jurisdiction>_<account>_<secret>_<random>", errMalformedSecret)
	}
	jurisdiction, account, secretID, tail := parts[0], parts[1], parts[2], parts[3]
	if !isJurisdiction(jurisdiction) {
		return parsedSecret{}, fmt.Errorf("%w: invalid jurisdiction", errMalformedSecret)
	}
	if !isULID(account) || !isULID(secretID) {
		return parsedSecret{}, fmt.Errorf("%w: invalid ID", errMalformedSecret)
	}
	if len(tail) != secretRandomLen+secretCheckLen {
		return parsedSecret{}, fmt.Errorf("%w: wrong length", errMalformedSecret)
	}
	random, check := tail[:secretRandomLen], tail[secretRandomLen:]
	if !onlyFrom(random, base32Lower) {
		return parsedSecret{}, fmt.Errorf("%w: invalid character", errMalformedSecret)
	}
	// The last character carries 4 unused bits. Requiring them to be zero,
	// as Core does, gives each secret exactly one spelling.
	decoded, err := secretBase32.DecodeString(random)
	if err != nil || secretBase32.EncodeToString(decoded) != random {
		return parsedSecret{}, fmt.Errorf("%w: invalid character", errMalformedSecret)
	}
	if check != secretChecksum(secret[:len(secret)-secretCheckLen]) {
		return parsedSecret{}, fmt.Errorf("%w: checksum mismatch; check for a typo or a truncated paste", errMalformedSecret)
	}
	return parsedSecret{jurisdiction: jurisdiction, accountID: "svc_" + account}, nil
}

// secretChecksum returns the CRC-32 (IEEE) of body in 6 base62 digits, most
// significant first.
func secretChecksum(body string) string {
	n := crc32.ChecksumIEEE([]byte(body))
	var out [secretCheckLen]byte
	for i := secretCheckLen - 1; i >= 0; i-- {
		out[i] = base62Alphabet[n%62]
		n /= 62
	}
	return string(out[:])
}

// isJurisdiction reports whether s matches ^[a-z]{2,8}$.
func isJurisdiction(s string) bool {
	if len(s) < 2 || len(s) > 8 {
		return false
	}
	for i := range len(s) {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

// isULID reports whether s is a ULID as ulid.String() prints it: 26
// uppercase Crockford base32 characters whose first fits in 3 bits.
func isULID(s string) bool {
	return len(s) == ulidLen && s[0] <= '7' && onlyFrom(s, crockford)
}

func onlyFrom(s, alphabet string) bool {
	for i := range len(s) {
		if strings.IndexByte(alphabet, s[i]) < 0 {
			return false
		}
	}
	return true
}

// secretCoreOrigin returns the Core that exchanges a secret from jurisdiction: the
// Core of that jurisdiction under the default Core's domain.
func secretCoreOrigin(jurisdiction string) *url.URL {
	_, domain, ok := corehost.Split(mustParse(defaultCoreURL).Hostname())
	if !ok {
		panic("entire: the default Core is not named like a production Core")
	}
	return corehost.Origin(jurisdiction, domain)
}

// Subject token types of the exchange at Core's /oauth/token.
const (
	tokenTypeAccessToken = "urn:ietf:params:oauth:token-type:access_token"
	tokenTypeJWT         = "urn:ietf:params:oauth:token-type:jwt"
)

// tokenExchanger trades a subject token, a service account secret or an
// OIDC ID token, for service account access tokens at Core, and caches the
// current one. It is safe for concurrent use.
type tokenExchanger struct {
	// subject returns the subject token for one exchange, and its type.
	subject   func(ctx context.Context) (token, tokenType string, err error)
	accountID string
	// authOrigin is the Core origin that runs /oauth/token. The issued
	// token names this Core, so routing then follows it.
	authOrigin *url.URL
	httpClient *http.Client
	userAgent  string
	now        func() time.Time

	// exchanging holds one slot, so concurrent requests share one exchange.
	// A waiter gives up when its own context ends, even if the exchange in
	// flight stalls.
	exchanging chan struct{}

	// mu guards token and renewAt. It is never held across I/O.
	mu    sync.Mutex
	token string
	// renewAt is when the cached token is next exchanged again.
	renewAt time.Time
}

func newTokenExchanger(accountID string, core *url.URL, httpClient *http.Client, userAgent string, subject func(context.Context) (string, string, error)) *tokenExchanger {
	return &tokenExchanger{
		subject:    subject,
		accountID:  accountID,
		authOrigin: &url.URL{Scheme: core.Scheme, Host: core.Host},
		httpClient: httpClient,
		userAgent:  userAgent,
		now:        time.Now,
		exchanging: make(chan struct{}, 1),
	}
}

func newSecretExchanger(secret string, core *url.URL, httpClient *http.Client, userAgent string) (*tokenExchanger, error) {
	parsed, err := parseServiceAccountSecret(secret)
	if err != nil {
		return nil, err
	}
	if core == nil {
		core = secretCoreOrigin(parsed.jurisdiction)
	}
	return newTokenExchanger(parsed.accountID, core, httpClient, userAgent, func(context.Context) (string, string, error) {
		return secret, tokenTypeAccessToken, nil
	}), nil
}

// tokenSource returns a cached access token, or exchanges the subject for a
// new one. Concurrent requests share one exchange.
func (e *tokenExchanger) tokenSource(ctx context.Context) (string, error) {
	if token, ok := e.cached(); ok {
		return token, nil
	}
	select {
	case e.exchanging <- struct{}{}:
	case <-ctx.Done():
		return "", fmt.Errorf("entire: wait for service account token exchange: %w", ctx.Err())
	}
	defer func() { <-e.exchanging }()
	// The exchange this request waited for may have renewed the token.
	if token, ok := e.cached(); ok {
		return token, nil
	}
	token, ttl, err := e.exchange(ctx)
	if err != nil {
		return "", err
	}
	e.mu.Lock()
	e.token, e.renewAt = token, e.now().Add(ttl-min(secretRefreshMargin, ttl/2))
	e.mu.Unlock()
	return token, nil
}

// cached returns the current token while it is fresh.
func (e *tokenExchanger) cached() (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.token, e.token != "" && e.now().Before(e.renewAt)
}

// reject drops token from the cache after the API refused it with 401, so
// the next request exchanges again. A newer token stays.
func (e *tokenExchanger) reject(token string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.token == token {
		e.token, e.renewAt = "", time.Time{}
	}
}

func (e *tokenExchanger) exchange(ctx context.Context) (string, time.Duration, error) {
	subject, subjectType, err := e.subject(ctx)
	if err != nil {
		return "", 0, err
	}
	form := url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token_type": {subjectType},
		"subject_token":      {subject},
		"audience":           {e.authOrigin.String()},
		"resource":           {e.accountID},
	}
	endpoint := e.authOrigin.JoinPath("oauth", "token")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("entire: build service account token exchange: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", e.userAgent)
	req.SetBasicAuth(secretClientID, "")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("entire: service account token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
	if err != nil {
		return "", 0, fmt.Errorf("entire: read service account token exchange: %w", err)
	}
	var out struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int64  `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode != http.StatusOK {
		// Never echo the request: it holds the subject token.
		if out.Error != "" {
			return "", 0, fmt.Errorf("entire: service account token exchange: %s: %s", out.Error, out.ErrorDescription)
		}
		return "", 0, fmt.Errorf("entire: service account token exchange: HTTP %d", resp.StatusCode)
	}
	if out.AccessToken == "" {
		return "", 0, errors.New("entire: service account token exchange returned no access token")
	}
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = secretFallbackTTL
	}
	return out.AccessToken, ttl, nil
}
