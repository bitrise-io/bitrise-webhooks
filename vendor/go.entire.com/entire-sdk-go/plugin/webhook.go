package plugin

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	entire "go.entire.com/entire-sdk-go"
)

// Webhooks are experimental: the delivery contract is not final, and field
// names may still change.

const (
	// maxDeliveryBytes bounds a webhook body.
	maxDeliveryBytes = 5 << 20
	// deliveryRetention is how long handled deliveries are remembered:
	// Entire keeps delivery history, and can resend, for three days.
	deliveryRetention = 72 * time.Hour
)

// Ref event types. Decode their payload with Event.RefChange.
const (
	EventRefCreated = "ref.created"
	// EventRefUpdated is also sent for a force push, with Forced set.
	EventRefUpdated = "ref.updated"
	EventRefDeleted = "ref.deleted"
)

// Installation event types. Decode their payload with
// Event.InstallationChange.
const (
	EventInstallationCreated             = "installation.created"
	EventInstallationUpdated             = "installation.updated"
	EventInstallationDeleted             = "installation.deleted"
	EventInstallationRepositoriesAdded   = "installation.repositories_added"
	EventInstallationRepositoriesRemoved = "installation.repositories_removed"
	EventInstallationSuspended           = "installation.suspended"
	EventInstallationResumed             = "installation.resumed"
)

// Trail event types. Decode their payload with Event.TrailChange. Entire
// sends them only to an installation that holds contents or trails
// permission on the trail's repository.
const (
	// EventTrailOpened is sent when a trail opens, as a draft or ready.
	// Opening a trail pushes nothing, so no ref event may follow.
	EventTrailOpened = "trail.opened"
	// EventTrailUpdated is sent when a draft or open trail changes its draft
	// state, title, branch, base, or head commit.
	EventTrailUpdated  = "trail.updated"
	EventTrailClosed   = "trail.closed"
	EventTrailReopened = "trail.reopened"
	EventTrailMerged   = "trail.merged"
)

// Delivery is one webhook delivery from Entire.
//
// Experimental: field names may change before the contract is final.
type Delivery struct {
	// ID identifies the delivery. It stays the same when Entire retries or
	// resends it.
	ID string `json:"delivery_id"`
	// PluginID is the plugin slug.
	PluginID string `json:"plugin_id"`
	// InstallationID is the installation the event belongs to.
	InstallationID string `json:"installation_id"`
	// CoreURL is the installation's Core. Store it with the installation
	// when it changes.
	CoreURL string `json:"core_url"`
	// APIURL is the API host of the event's repository, for check runs, if
	// the event concerns one.
	APIURL string `json:"api_url"`
	// Event is what happened.
	Event Event `json:"event"`
}

// Event is something that happened to an installation or its repositories.
type Event struct {
	// ID identifies the event.
	ID string `json:"id"`
	// Type is the event type, such as EventRefUpdated.
	Type string `json:"type"`
	// Time is when the event happened.
	Time entire.Timestamp `json:"time"`
	// Payload is the event's data. Decode it with RefChange,
	// InstallationChange, or TrailChange, by Type, or with Decode.
	Payload json.RawMessage `json:"payload"`
}

// Decode decodes the payload into v.
func (e Event) Decode(v any) error {
	if err := json.Unmarshal(e.Payload, v); err != nil {
		return fmt.Errorf("plugin: decode %s payload: %w", e.Type, err)
	}
	return nil
}

// RefChange decodes the payload of a ref.created, ref.updated, or
// ref.deleted event.
func (e Event) RefChange() (*RefChange, error) {
	if !strings.HasPrefix(e.Type, "ref.") {
		return nil, fmt.Errorf("plugin: a %s event is not a ref change", e.Type)
	}
	var change RefChange
	if err := e.Decode(&change); err != nil {
		return nil, err
	}
	return &change, nil
}

// RefChange is a branch or tag that changed. Each event carries one ref.
type RefChange struct {
	// Repository is the repository whose ref changed.
	Repository EventRepository `json:"repository"`
	// Ref is the full ref name, such as "refs/heads/main".
	Ref string `json:"ref"`
	// Before is the commit the ref pointed to: all zeros for a created
	// ref. IsZeroSHA reports that.
	Before string `json:"before"`
	// After is the commit the ref points to: all zeros for a deleted ref.
	After string `json:"after"`
	// Forced reports a force push, on ref.updated.
	Forced bool `json:"forced"`
	// Pusher is who pushed. It is the zero Pusher when Entire does not
	// know.
	Pusher Pusher `json:"pusher"`
}

// Branch returns the branch name of a refs/heads/ ref, such as "main", and
// whether the ref is a branch.
func (c *RefChange) Branch() (string, bool) {
	return strings.CutPrefix(c.Ref, "refs/heads/")
}

// IsZeroSHA reports whether sha is the all-zeros object ID that marks a
// created or deleted ref.
func IsZeroSHA(sha string) bool {
	return strings.Trim(sha, "0") == "" && (len(sha) == 40 || len(sha) == 64)
}

// EventRepository is the repository an event concerns.
type EventRepository struct {
	// ID identifies the repository.
	ID string `json:"id"`
	// Path is the repository's path on its Entire host, such as
	// "/et/acme/api".
	Path string `json:"path"`
	// DefaultBranch is the full ref of the default branch, such as
	// "refs/heads/main". It may be empty.
	DefaultBranch string `json:"default_branch"`
	// CloneURL clones the repository over HTTPS.
	CloneURL string `json:"clone_url"`
	// EntireCloneURL clones the repository with Entire's Git remote
	// helper, such as "entire://aws-us-east-2.entire.com/et/acme/api".
	EntireCloneURL string `json:"entire_clone_url"`
}

// Pusher is the account that pushed.
type Pusher struct {
	Account struct {
		// ID identifies the account.
		ID string `json:"id"`
		// Handle is the account's handle.
		Handle string `json:"handle"`
	} `json:"account"`
}

// InstallationChange decodes the payload of an installation event, such as
// installation.created or installation.suspended.
func (e Event) InstallationChange() (*InstallationChange, error) {
	if !strings.HasPrefix(e.Type, "installation.") {
		return nil, fmt.Errorf("plugin: a %s event is not an installation change", e.Type)
	}
	var change InstallationChange
	if err := e.Decode(&change); err != nil {
		return nil, err
	}
	return &change, nil
}

// InstallationChange is an installation as it stands after a change.
type InstallationChange struct {
	Installation InstallationState `json:"installation"`
}

// InstallationState is an installation's state. The delivery names the
// installation; this is what it covers.
type InstallationState struct {
	// ProjectID is the project the plugin is installed on.
	ProjectID string `json:"project_id"`
	// Project is the project's public identity. It is nil when Entire
	// cannot resolve it; key on ProjectID, which never changes.
	Project *Project `json:"project"`
	// Revision increases with each change to the installation. Keep the
	// highest one applied, and ignore an event at or below it. After a gap
	// in revisions, read the repository list with
	// Installation.Repositories.
	Revision int64 `json:"revision"`
	// RepositorySelection is "all" or "selected", on created and updated
	// events.
	RepositorySelection string `json:"repository_selection"`
	// RepositoryCount is how many repositories the installation covers, on
	// created and updated events.
	RepositoryCount int `json:"repository_count"`
	// RepositoryIDs are the repositories the installation covers, when the
	// event carries the list.
	RepositoryIDs []string `json:"repository_ids"`
	// RepositoriesOmitted reports a list too long to include. Read it with
	// Installation.Repositories.
	RepositoriesOmitted bool `json:"repositories_omitted"`
	// AddedRepositoryIDs are the repositories an
	// installation.repositories_added event added.
	AddedRepositoryIDs []string `json:"added_repository_ids"`
	// RemovedRepositoryIDs are the repositories an
	// installation.repositories_removed event removed.
	RemovedRepositoryIDs []string `json:"removed_repository_ids"`
	// SuspendedBy is SuspendedByPlugin or SuspendedByCustomer, on
	// installation.suspended. Only the side that suspended an installation
	// can resume it.
	SuspendedBy SuspendedBy `json:"suspended_by"`
	// ManageURL is the page where the customer manages the installation. It
	// is empty on installation.deleted, and when Entire cannot build it.
	ManageURL string `json:"manage_url"`
}

// TrailChange decodes the payload of a trail event, such as trail.opened or
// trail.merged.
func (e Event) TrailChange() (*TrailChange, error) {
	if !strings.HasPrefix(e.Type, "trail.") {
		return nil, fmt.Errorf("plugin: a %s event is not a trail change", e.Type)
	}
	var change TrailChange
	if err := e.Decode(&change); err != nil {
		return nil, err
	}
	return &change, nil
}

// TrailChange is a trail as it stands after a change, and what changed.
// Entire does not order deliveries: compare Trail.UpdatedAt with the state
// you hold, and ignore an older one.
type TrailChange struct {
	// Repository is the trail's repository. Its DefaultBranch is empty.
	Repository EventRepository `json:"repository"`
	// Trail is the trail after the change.
	Trail TrailState `json:"trail"`
	// Changes names the fields the event changed. It is nil on
	// trail.opened.
	Changes *TrailFieldChanges `json:"changes"`
	// Actor is who made the change. It is nil when Entire does not know,
	// such as for a push or a change on the forge.
	Actor *Actor `json:"actor"`
}

// TrailState is a trail's state.
type TrailState struct {
	// ID identifies the trail. It does not change.
	ID string `json:"id"`
	// Number is the trail's number in its repository.
	Number int `json:"number"`
	// Title is the trail's title.
	Title string `json:"title"`
	// State is entire.ChangeStatusDraft, ChangeStatusOpen,
	// ChangeStatusClosed, or ChangeStatusMerged.
	State entire.ChangeStatus `json:"state"`
	// Branch is the branch the trail tracks. It is empty for a trail with
	// no branch, or once the trail released its branch.
	Branch string `json:"branch"`
	// Base is the branch the trail merges into.
	Base string `json:"base"`
	// HeadSHA is the commit Entire last recorded for the branch. It is
	// empty until Entire records one; a later trail.updated sets it.
	HeadSHA string `json:"head_sha"`
	// Author is who opened the trail, or nil when Entire does not know.
	Author    *Actor           `json:"author"`
	CreatedAt entire.Timestamp `json:"created_at"`
	UpdatedAt entire.Timestamp `json:"updated_at"`
	// MergedAt is set on a merged trail only.
	MergedAt entire.Timestamp `json:"merged_at"`
}

// TrailFieldChanges names the fields a trail event changed. A nil field did
// not change.
type TrailFieldChanges struct {
	State   *FieldChange `json:"state"`
	Title   *FieldChange `json:"title"`
	Branch  *FieldChange `json:"branch"`
	Base    *FieldChange `json:"base"`
	HeadSHA *FieldChange `json:"head_sha"`
}

// FieldChange is a field's value before and after a change. Either is empty
// when there was no value.
type FieldChange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Actor is an account that acted.
type Actor struct {
	Account struct {
		// ID identifies the account.
		ID string `json:"id"`
	} `json:"account"`
}

// DeliveryError reports a webhook request that failed verification. The
// handler answers it with Status: a 4xx, which tells Entire not to retry,
// or 503 when the delivery keys could not be fetched, which Entire retries.
type DeliveryError struct {
	// Status is the HTTP status to answer with.
	Status int
	// Err is the reason.
	Err error
}

func (e *DeliveryError) Error() string { return "plugin: rejected delivery: " + e.Err.Error() }

// Unwrap returns the reason.
func (e *DeliveryError) Unwrap() error { return e.Err }

type deliveryClaims struct {
	Issuer     string      `json:"iss"`
	Audience   audience    `json:"aud"`
	IssuedAt   numericDate `json:"iat"`
	ExpiresAt  numericDate `json:"exp"`
	JTI        string      `json:"jti"`
	BodySHA256 string      `json:"body_sha256"`
}

// VerifyDelivery reads and verifies a webhook request from Entire, and
// decodes it. It checks the delivery JWT in the Authorization header
// against the delivery JWKS (EdDSA only), its issuer (see
// Config.DeliveryIssuer), audience
// (Config.DeliveryAudience, the plugin slug by default), times, and
// that its ID was not seen before, and compares its body_sha256 with a
// digest of the raw body before parsing anything. Failures are
// *DeliveryError. It does not skip deliveries already handled; WebhookHandler
// does.
//
// Experimental: the delivery contract is not final.
func (a *App) VerifyDelivery(r *http.Request) (*Delivery, error) {
	reject := func(status int, err error) (*Delivery, error) {
		return nil, &DeliveryError{Status: status, Err: err}
	}
	if a.cfg.DeliveryJWKSURL == "" {
		return nil, errors.New("plugin: Config.DeliveryJWKSURL is required for webhooks")
	}
	if r.Method != http.MethodPost {
		return reject(http.StatusMethodNotAllowed, fmt.Errorf("method %s", r.Method))
	}
	bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || bearer == "" {
		return reject(http.StatusUnauthorized, errors.New("no bearer token"))
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxDeliveryBytes+1))
	if err != nil {
		return reject(http.StatusBadRequest, fmt.Errorf("read body: %w", err))
	}
	if len(body) > maxDeliveryBytes {
		return reject(http.StatusRequestEntityTooLarge, fmt.Errorf("body exceeds %d bytes", maxDeliveryBytes))
	}
	token, err := parseJWT(bearer)
	if err != nil {
		return reject(http.StatusUnauthorized, err)
	}
	key, err := a.deliveryKeySet().key(r.Context(), token.header.Kid)
	if errors.Is(err, errKeysUnavailable) {
		// Entire retries a 503, by when the keys may be reachable.
		return reject(http.StatusServiceUnavailable, err)
	}
	if err != nil {
		return reject(http.StatusUnauthorized, err)
	}
	if err := token.verify(key); err != nil {
		return reject(http.StatusUnauthorized, err)
	}
	var claims deliveryClaims
	if err := json.Unmarshal(token.payload, &claims); err != nil {
		return reject(http.StatusUnauthorized, err)
	}
	switch {
	case !a.deliveryIssuer(claims.Issuer):
		return reject(http.StatusUnauthorized, fmt.Errorf("iss is %q", claims.Issuer))
	case !claims.Audience.contains(a.cfg.DeliveryAudience):
		return reject(http.StatusUnauthorized, fmt.Errorf("aud does not include %q", a.cfg.DeliveryAudience))
	case claims.IssuedAt == 0:
		return reject(http.StatusUnauthorized, errors.New("no iat"))
	case claims.JTI == "":
		return reject(http.StatusUnauthorized, errors.New("no jti"))
	}
	if err := checkTimes(claims.IssuedAt, claims.ExpiresAt, a.now()); err != nil {
		return reject(http.StatusUnauthorized, err)
	}
	digest := sha256.Sum256(body)
	if subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(digest[:])), []byte(claims.BodySHA256)) != 1 {
		return reject(http.StatusUnauthorized, errors.New("body_sha256 does not match the body"))
	}
	fresh, err := a.cfg.ReplayStore.Remember(r.Context(), "delivery:"+claims.JTI, claims.ExpiresAt.time().Add(leeway))
	if err != nil {
		return nil, fmt.Errorf("plugin: record delivery JWT: %w", err)
	}
	if !fresh {
		return reject(http.StatusUnauthorized, errors.New("delivery JWT reused"))
	}
	var delivery Delivery
	if err := json.Unmarshal(body, &delivery); err != nil {
		return reject(http.StatusBadRequest, fmt.Errorf("decode body: %w", err))
	}
	if delivery.PluginID != a.cfg.Slug {
		return reject(http.StatusBadRequest, fmt.Errorf("plugin_id is %q", delivery.PluginID))
	}
	if delivery.ID == "" || delivery.InstallationID == "" {
		return reject(http.StatusBadRequest, errors.New("no delivery_id or installation_id"))
	}
	a.noteDelivery(&delivery)
	return &delivery, nil
}

// noteDelivery refreshes the cached API host of the delivery's repository.
func (a *App) noteDelivery(d *Delivery) {
	a.mu.Lock()
	inst := a.installations[d.InstallationID]
	a.mu.Unlock()
	if inst == nil || !(strings.HasPrefix(d.Event.Type, "ref.") || strings.HasPrefix(d.Event.Type, "trail.")) {
		return
	}
	var payload struct {
		Repository EventRepository `json:"repository"`
	}
	if json.Unmarshal(d.Event.Payload, &payload) != nil {
		return
	}
	if _, err := a.checkOrigin(d.APIURL); err == nil {
		inst.noteAPIURL(payload.Repository.ID, d.APIURL)
	}
}

// WebhookHandler returns the handler for the plugin's webhook URL. It
// verifies each request with VerifyDelivery, skips deliveries already
// handled, and calls handle. Entire retries a delivery that fails with a
// transport error, 429, or 5xx, so the handler answers:
//
//   - 200 when handle succeeds, or the delivery was already handled;
//   - the DeliveryError's status when verification fails: a 4xx, which
//     Entire does not retry, or 503 when the delivery keys are unreachable;
//   - 500 when handle returns an error, so that Entire retries.
//
// Entire expects an answer within seconds: hand slow work to a queue and
// return.
//
// Experimental: the delivery contract is not final.
func (a *App) WebhookHandler(handle func(ctx context.Context, d *Delivery) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delivery, err := a.VerifyDelivery(r)
		var rejected *DeliveryError
		switch {
		case errors.As(err, &rejected):
			http.Error(w, http.StatusText(rejected.Status), rejected.Status)
			return
		case err != nil:
			a.logger.ErrorContext(r.Context(), "plugin: webhook", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ctx := r.Context()
		if handled, err := a.cfg.DeliveryStore.Handled(ctx, delivery.ID); err != nil {
			a.logger.ErrorContext(ctx, "plugin: webhook", "delivery", delivery.ID, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		} else if handled {
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := handle(ctx, delivery); err != nil {
			a.logger.ErrorContext(ctx, "plugin: webhook", "delivery", delivery.ID, "event", delivery.Event.Type, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := a.cfg.DeliveryStore.MarkHandled(ctx, delivery.ID, a.now().Add(deliveryRetention)); err != nil {
			a.logger.ErrorContext(ctx, "plugin: webhook", "delivery", delivery.ID, "error", err)
		}
		w.WriteHeader(http.StatusOK)
	})
}

// legacyDeliveryIssuers are the iss of deliveries from a cell that has no
// public URL of its own: the web app's origin, on either of Entire's domains.
var legacyDeliveryIssuers = []string{"https://entire.com", "https://entire.io"}

// deliveryIssuer reports whether iss may sign deliveries. The keys come from
// Config.DeliveryJWKSURL whatever iss says, so iss only has to name Entire.
func (a *App) deliveryIssuer(iss string) bool {
	if a.cfg.DeliveryIssuer != "" {
		return iss == a.cfg.DeliveryIssuer
	}
	if slices.Contains(legacyDeliveryIssuers, iss) {
		return true
	}
	origin, err := a.checkOrigin(iss)
	return err == nil && origin.Scheme == "https" && origin.String() == iss
}

func (a *App) deliveryKeySet() *keySet {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.deliveryKeys == nil {
		a.deliveryKeys = &keySet{url: a.cfg.DeliveryJWKSURL, http: a.httpClient, now: a.now}
	}
	return a.deliveryKeys
}
