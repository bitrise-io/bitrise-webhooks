package plugin

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// receiptType is the typ header of an install receipt.
const receiptType = "entire-install-receipt+jwt"

// ErrAccessDenied reports that the customer cancelled the install.
var ErrAccessDenied = errors.New("plugin: the customer cancelled the install")

// ErrStateMismatch reports a callback whose state is not the one sent for
// this session. Treat it as a forged or replayed callback.
var ErrStateMismatch = errors.New("plugin: install state does not match this session")

// ReceiptAction says what a customer's consent did.
type ReceiptAction string

// Receipt actions.
const (
	// ActionInstall is a new installation, or a retried consent that
	// finished one.
	ActionInstall ReceiptAction = "install"
	// ActionUpdate is a change to an installation the plugin may already
	// hold, such as a new repository selection or accepted permissions.
	ActionUpdate ReceiptAction = "update"
)

// Receipt is a verified install receipt: proof that a customer installed
// the plugin on a project. Store InstallationID and CoreURL together; they
// are all the plugin needs to act for the installation later.
type Receipt struct {
	// InstallationID identifies the installation.
	InstallationID string
	// CoreURL is the Core that handled the consent and issued the receipt,
	// such as "https://us.auth.entire.io": the Core of the project's
	// jurisdiction. Installations are global, so it is only the best Core
	// to call; pass it to App.Installation.
	CoreURL string
	// Action is ActionInstall or ActionUpdate, or empty from a Core that
	// does not send it. An update names an installation the plugin may
	// already hold: update it rather than adding another.
	Action ReceiptAction
	// Org and OrgID are always empty: plugins install only on projects, and
	// Core no longer sends these claims.
	//
	// Deprecated: Use ProjectID. Org and OrgID will be removed in a later
	// release.
	Org, OrgID string
	// Project is the name of the project the plugin was installed on.
	Project string
	// ProjectID identifies the project. It never changes; store it rather
	// than a name.
	ProjectID string
	// ProjectName is the project's public name, as it appears in Entire's
	// URLs: the project's name on Entire, or the GitHub owner of a GitHub
	// project. It is empty when Entire cannot resolve it.
	ProjectName string
	// Forge is where the project's repositories live, ForgeEntire or
	// ForgeGitHub, and is empty with ProjectName.
	Forge Forge
	// ManageURL is the page where the customer manages the installation.
	// It is empty when Entire cannot build it.
	ManageURL string
	// ID is the receipt's unique ID (its jti).
	ID string
	// IssuedAt is when Entire issued the receipt.
	IssuedAt time.Time
	// ExpiresAt is when the receipt expires.
	ExpiresAt time.Time
}

type receiptClaims struct {
	Issuer         string        `json:"iss"`
	Audience       audience      `json:"aud"`
	Subject        string        `json:"sub"`
	InstallationID string        `json:"installation_id"`
	PluginID       string        `json:"plugin_id"`
	Action         ReceiptAction `json:"action"`
	Project        string        `json:"project"`
	ProjectID      string        `json:"project_id"`
	ProjectName    string        `json:"project_name"`
	Forge          Forge         `json:"forge"`
	ManageURL      string        `json:"manage_url"`
	State          string        `json:"state"`
	JTI            string        `json:"jti"`
	IssuedAt       numericDate   `json:"iat"`
	ExpiresAt      numericDate   `json:"exp"`
}

// AcceptReceipt handles the query of a request to the plugin's install
// callback. It returns ErrAccessDenied when the customer cancelled,
// ErrStateMismatch when state is not expectedState, and otherwise verifies
// the receipt with VerifyReceipt.
func (a *App) AcceptReceipt(ctx context.Context, query url.Values, expectedState string) (*Receipt, error) {
	if !sameState(query.Get("state"), expectedState) {
		return nil, ErrStateMismatch
	}
	if code := query.Get("error"); code != "" {
		if code == "access_denied" {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("plugin: install failed: %s", code)
	}
	receipt := query.Get("receipt")
	if receipt == "" {
		return nil, errors.New("plugin: install callback has no receipt")
	}
	return a.VerifyReceipt(ctx, receipt, expectedState)
}

// VerifyReceipt verifies an install receipt as the plugin guide requires:
// its type, an issuer in Config.Cores, its signature against the issuer's
// JWKS, its audience (Config.Audience), plugin, expiry, and state, and that
// its ID was not accepted before.
func (a *App) VerifyReceipt(ctx context.Context, receipt, expectedState string) (*Receipt, error) {
	if a.cfg.Audience == "" {
		return nil, errors.New("plugin: Config.Audience is required to accept receipts")
	}
	fail := func(err error) (*Receipt, error) { return nil, fmt.Errorf("plugin: invalid install receipt: %w", err) }
	token, err := parseJWT(receipt)
	if err != nil {
		return fail(err)
	}
	if token.header.Typ != receiptType {
		return fail(fmt.Errorf("typ is %q, not %s", token.header.Typ, receiptType))
	}
	var claims receiptClaims
	if err := json.Unmarshal(token.payload, &claims); err != nil {
		return fail(err)
	}
	// The issuer chooses the keys, so check it before fetching them.
	core, err := a.checkCore(claims.Issuer)
	if err != nil {
		return fail(err)
	}
	key, err := a.receiptKeySet(core.JoinPath(".well-known", "jwks.json").String()).key(ctx, token.header.Kid)
	if err != nil {
		return fail(err)
	}
	if err := token.verify(key); err != nil {
		return fail(err)
	}
	now := a.now()
	switch {
	case !claims.Audience.contains(a.cfg.Audience):
		return fail(fmt.Errorf("aud does not include %q", a.cfg.Audience))
	case claims.PluginID != a.cfg.Slug:
		return fail(fmt.Errorf("plugin_id is %q, not %q", claims.PluginID, a.cfg.Slug))
	case claims.InstallationID == "":
		return fail(errors.New("no installation_id"))
	case claims.Subject != "" && claims.Subject != claims.InstallationID:
		return fail(errors.New("sub and installation_id differ"))
	case !sameState(claims.State, expectedState):
		return nil, ErrStateMismatch
	case claims.JTI == "":
		return fail(errors.New("no jti"))
	}
	if err := checkTimes(claims.IssuedAt, claims.ExpiresAt, now); err != nil {
		return fail(err)
	}
	fresh, err := a.cfg.ReplayStore.Remember(ctx, "receipt:"+claims.JTI, claims.ExpiresAt.time().Add(leeway))
	if err != nil {
		return nil, fmt.Errorf("plugin: record receipt: %w", err)
	}
	if !fresh {
		return fail(errors.New("already accepted"))
	}
	return &Receipt{
		InstallationID: claims.InstallationID,
		CoreURL:        core.String(),
		Action:         claims.Action,
		Project:        claims.Project,
		ProjectID:      claims.ProjectID,
		ProjectName:    claims.ProjectName,
		Forge:          claims.Forge,
		ManageURL:      claims.ManageURL,
		ID:             claims.JTI,
		IssuedAt:       claims.IssuedAt.time(),
		ExpiresAt:      claims.ExpiresAt.time(),
	}, nil
}

// sameState compares states in constant time. An empty expected state never
// matches.
func sameState(got, expected string) bool {
	return expected != "" && subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
