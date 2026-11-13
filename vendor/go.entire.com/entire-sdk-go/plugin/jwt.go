package plugin

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// leeway is the clock skew allowed when checking a JWT's times.
const leeway = time.Minute

const maxJWKSBytes = 1 << 20

// jwtHeader is the JOSE header of a JWT.
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid,omitempty"`
	Typ string `json:"typ,omitempty"`
}

// signJWT signs claims as an EdDSA JWT with signer.
func signJWT(signer crypto.Signer, kid string, claims any) (string, error) {
	header, err := json.Marshal(jwtHeader{Alg: "EdDSA", Kid: kid, Typ: "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	signature, err := signer.Sign(rand.Reader, []byte(input), crypto.Hash(0))
	if err != nil {
		return "", fmt.Errorf("plugin: sign JWT: %w", err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// parsedJWT is a JWT split into its parts. Nothing in it is verified.
type parsedJWT struct {
	header    jwtHeader
	payload   []byte
	input     []byte
	signature []byte
}

func parseJWT(token string) (*parsedJWT, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("not a JWT")
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("JWT header is not base64url")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("JWT payload is not base64url")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("JWT signature is not base64url")
	}
	p := &parsedJWT{payload: payload, input: []byte(parts[0] + "." + parts[1]), signature: signature}
	if err := json.Unmarshal(header, &p.header); err != nil {
		return nil, errors.New("JWT header is not JSON")
	}
	if p.header.Alg != "EdDSA" {
		return nil, fmt.Errorf("JWT algorithm %q is not EdDSA", p.header.Alg)
	}
	if p.header.Kid == "" {
		return nil, errors.New("JWT header has no kid")
	}
	return p, nil
}

// verify checks the signature against key.
func (p *parsedJWT) verify(key ed25519.PublicKey) error {
	if !ed25519.Verify(key, p.input, p.signature) {
		return errors.New("JWT signature is invalid")
	}
	return nil
}

// audience is a JWT aud claim: one string or an array of them.
type audience []string

func (a *audience) UnmarshalJSON(data []byte) error {
	var one string
	if json.Unmarshal(data, &one) == nil {
		*a = audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return errors.New("aud is neither a string nor an array of strings")
	}
	*a = many
	return nil
}

func (a audience) contains(want string) bool {
	for _, v := range a {
		if v == want {
			return true
		}
	}
	return false
}

// numericDate is a JWT time in seconds since the epoch. Fractions are
// accepted and dropped.
type numericDate int64

func (n *numericDate) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return errors.New("JWT time is not a number")
	}
	*n = numericDate(f)
	return nil
}

func (n numericDate) time() time.Time { return time.Unix(int64(n), 0) }

// checkTimes checks exp, and iat when present, against now with leeway.
func checkTimes(iat, exp numericDate, now time.Time) error {
	if exp == 0 {
		return errors.New("JWT has no exp")
	}
	if now.After(exp.time().Add(leeway)) {
		return errors.New("JWT has expired")
	}
	if iat != 0 && iat.time().After(now.Add(leeway)) {
		return errors.New("JWT was issued in the future")
	}
	return nil
}

// PublicKey is an Ed25519 public key published in the plugin's JWKS.
type PublicKey struct {
	// KeyID is the key's kid.
	KeyID string
	// Key is the public key.
	Key ed25519.PublicKey
}

// jwk is one JSON Web Key.
type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	X   string `json:"x"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
}

// marshalJWKS encodes keys as a JWKS document.
func marshalJWKS(keys []PublicKey) ([]byte, error) {
	doc := struct {
		Keys []jwk `json:"keys"`
	}{Keys: []jwk{}}
	for _, k := range keys {
		doc.Keys = append(doc.Keys, jwk{Kty: "OKP", Crv: "Ed25519", Kid: k.KeyID, X: base64.RawURLEncoding.EncodeToString(k.Key), Use: "sig", Alg: "EdDSA"})
	}
	return json.Marshal(doc)
}

// parseJWKS decodes the Ed25519 signing keys of a JWKS document. Other keys
// are skipped.
func parseJWKS(data []byte) (map[string]ed25519.PublicKey, error) {
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}
	keys := map[string]ed25519.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Kid == "" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			continue
		}
		keys[k.Kid] = ed25519.PublicKey(x)
	}
	return keys, nil
}

const (
	// jwksTTL is how long fetched keys are used before fetching again.
	jwksTTL = time.Hour
	// jwksRefetchInterval limits fetches made for an unknown kid.
	jwksRefetchInterval = 30 * time.Second
)

// errKeysUnavailable marks a JWKS that could not be fetched, which a later
// attempt may fix.
var errKeysUnavailable = errors.New("signing keys unavailable")

// keySet caches the keys of one JWKS URL. It is safe for concurrent use.
type keySet struct {
	url  string
	http *http.Client
	now  func() time.Time

	mu       sync.Mutex
	keys     map[string]ed25519.PublicKey
	fetched  time.Time
	fetching *fetchCall
}

// fetchCall is a JWKS fetch that concurrent callers share.
type fetchCall struct {
	done chan struct{}
	keys map[string]ed25519.PublicKey
	err  error
}

// key returns the key for kid, fetching the JWKS when the cache is empty,
// stale, or lacks kid and was not fetched in the last 30 seconds. The fetch
// runs outside the lock, and concurrent callers share it, so a slow JWKS
// holds each caller only until its own context ends.
func (s *keySet) key(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	s.mu.Lock()
	now := s.now()
	key, ok := s.keys[kid]
	fresh := !s.fetched.IsZero() && now.Sub(s.fetched) < jwksTTL
	if ok && fresh {
		s.mu.Unlock()
		return key, nil
	}
	if !ok && fresh && now.Sub(s.fetched) < jwksRefetchInterval {
		s.mu.Unlock()
		return nil, fmt.Errorf("no key %q in %s", kid, s.url)
	}
	call := s.fetching
	if call == nil {
		call = &fetchCall{done: make(chan struct{})}
		s.fetching = call
		// The fetch runs apart from its callers, so that one caller's
		// cancellation does not fail the others.
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharedCallTimeout)
			defer cancel()
			call.keys, call.err = s.fetch(ctx)
			s.mu.Lock()
			s.fetching = nil
			if call.err == nil {
				s.keys, s.fetched = call.keys, now
			}
			s.mu.Unlock()
			close(call.done)
		}()
	}
	s.mu.Unlock()
	select {
	case <-call.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if call.err != nil {
		if ok {
			// Keep using a known key while the JWKS is unreachable.
			return key, nil
		}
		return nil, fmt.Errorf("%w: %w", errKeysUnavailable, call.err)
	}
	if key, ok = call.keys[kid]; !ok {
		return nil, fmt.Errorf("no key %q in %s", kid, s.url)
	}
	return key, nil
}

func (s *keySet) fetch(ctx context.Context) (map[string]ed25519.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch JWKS %s: HTTP %d", s.url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes+1))
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	if len(data) > maxJWKSBytes {
		return nil, fmt.Errorf("JWKS %s exceeds %d bytes", s.url, maxJWKSBytes)
	}
	return parseJWKS(bytes.TrimSpace(data))
}
