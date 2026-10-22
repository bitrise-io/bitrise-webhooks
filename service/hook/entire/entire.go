package entire

// # Infos / notes:
//
// ## Webhook calls
//
// Entire (https://entire.io) delivers signed events to a single webhook URL per
// plugin. The Bitrise Monolith owns that URL: it verifies the delivery JWT and
// forwards the event body, once per linked Bitrise app, to this service on
// `/h/entire/APP-SLUG/API-TOKEN` - the same scheme as the GitHub App forward.
//
// Because of this, the provider never sees Entire directly and does not verify
// signatures. It only accepts requests carrying the `X-Bitrise-Entire-Verified: 1`
// header the Monolith sets after verification; anything else is rejected.
//
// The forwarded body is Entire's delivery envelope, plus a `bitrise_enrichment`
// object added by the Monolith (commit message and author looked up through the
// Entire API, as the `ref.*` payloads only carry SHAs).
//
// ### Ref events
//
// `ref.created` and `ref.updated` carry the ref (`refs/heads/...` or `refs/tags/...`),
// the `before` / `after` SHAs and the pusher. One event is sent per ref.
// `ref.deleted` is acknowledged but never builds.
//
// ### Installation events
//
// `installation.*` events describe the plugin installation itself and are
// acknowledged without a build.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

// --------------------------
// --- Webhook Data Model ---

const (
	refCreatedEventType     = "ref.created"
	refUpdatedEventType     = "ref.updated"
	refDeletedEventType     = "ref.deleted"
	installationEventPrefix = "installation."

	branchRefPrefix = "refs/heads/"
	tagRefPrefix    = "refs/tags/"

	// VerifiedHeader is set by the Bitrise Monolith once it verified the delivery's signature.
	VerifiedHeader = "X-Bitrise-Entire-Verified"

	// ProviderID ...
	ProviderID = "entire"
)

// DeliveryModel is the envelope the Monolith forwards; `event.payload` is kept raw
// because its shape depends on `event.type`.
type DeliveryModel struct {
	DeliveryID string          `json:"delivery_id"`
	Event      EventModel      `json:"event"`
	Enrichment EnrichmentModel `json:"bitrise_enrichment"`
}

// EventModel ...
type EventModel struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// EnrichmentModel ...
type EnrichmentModel struct {
	CommitMessage string `json:"commit_message"`
	CommitAuthor  string `json:"commit_author"`
}

// RefEventPayloadModel ...
type RefEventPayloadModel struct {
	Repository RepositoryModel `json:"repository"`
	Ref        string          `json:"ref"`
	Before     string          `json:"before"`
	After      string          `json:"after"`
	Forced     bool            `json:"forced"`
	Pusher     *PusherModel    `json:"pusher"`
}

// RepositoryModel ...
type RepositoryModel struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
}

// PusherModel ...
type PusherModel struct {
	Account AccountModel `json:"account"`
}

// AccountModel ...
type AccountModel struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
}

// ---------------------------------------
// --- Webhook Provider Implementation ---

// HookProvider ...
type HookProvider struct{}

func detectContentType(header http.Header) (string, error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return "", errors.New("No Content-Type Header found")
	}
	return contentType, nil
}

func isZeroSHA(sha string) bool {
	return sha != "" && strings.Trim(sha, "0") == ""
}

func pusherHandle(payload RefEventPayloadModel) string {
	if payload.Pusher == nil {
		return ""
	}
	return payload.Pusher.Account.Handle
}

func transformRefEvent(eventType string, payload RefEventPayloadModel, enrichment EnrichmentModel) hookCommon.TransformResultModel {
	if eventType == refDeletedEventType {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Ref (%s) was deleted - ignoring", payload.Ref),
			ShouldSkip: true,
		}
	}

	if payload.Ref == "" {
		return hookCommon.TransformResultModel{
			Error: errors.New("Missing ref"),
		}
	}

	if isZeroSHA(payload.After) {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Ref (%s) was deleted ('after' is the zero SHA) - ignoring", payload.Ref),
			ShouldSkip: true,
		}
	}

	if payload.After == "" {
		return hookCommon.TransformResultModel{
			Error: errors.New("Missing commit hash ('after')"),
		}
	}

	buildParams := bitriseapi.BuildParamsModel{
		CommitHash:        payload.After,
		CommitMessage:     enrichment.CommitMessage,
		BaseRepositoryURL: payload.Repository.CloneURL,
		HeadRepositoryURL: payload.Repository.CloneURL,
	}

	switch {
	case strings.HasPrefix(payload.Ref, branchRefPrefix):
		buildParams.Branch = strings.TrimPrefix(payload.Ref, branchRefPrefix)
	case strings.HasPrefix(payload.Ref, tagRefPrefix):
		buildParams.Tag = strings.TrimPrefix(payload.Ref, tagRefPrefix)
	default:
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Ref (%s) is neither a branch nor a tag ref", payload.Ref),
		}
	}

	return hookCommon.TransformResultModel{
		TriggerAPIParams: []bitriseapi.TriggerAPIParamsModel{
			{
				BuildParams: buildParams,
				TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, pusherHandle(payload)),
			},
		},
	}
}

func transformDelivery(delivery DeliveryModel) hookCommon.TransformResultModel {
	eventType := delivery.Event.Type

	switch eventType {
	case refCreatedEventType, refUpdatedEventType, refDeletedEventType:
		var payload RefEventPayloadModel
		if len(delivery.Event.Payload) > 0 {
			if err := json.Unmarshal(delivery.Event.Payload, &payload); err != nil {
				return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse event payload: %s", err)}
			}
		}
		return transformRefEvent(eventType, payload, delivery.Enrichment)
	}

	if strings.HasPrefix(eventType, installationEventPrefix) {
		return hookCommon.TransformResultModel{
			Error:      fmt.Errorf("Installation event (%s) - nothing to build", eventType),
			ShouldSkip: true,
		}
	}

	return hookCommon.TransformResultModel{
		Error: fmt.Errorf("Unsupported Webhook event: %s", eventType),
	}
}

// TransformRequest ...
func (hp HookProvider) TransformRequest(r *http.Request) hookCommon.TransformResultModel {
	if verified := r.Header.Get(VerifiedHeader); verified != "1" {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Issue with Headers: %s header is missing or not '1' - only deliveries verified and forwarded by Bitrise are accepted", VerifiedHeader),
		}
	}

	contentType, err := detectContentType(r.Header)
	if err != nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Issue with Headers: %s", err),
		}
	}

	if contentType != hookCommon.ContentTypeApplicationJSON {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Content-Type is not supported: %s", contentType),
		}
	}

	if r.Body == nil {
		return hookCommon.TransformResultModel{
			Error: fmt.Errorf("Failed to read content of request body: no or empty request body"),
		}
	}

	var delivery DeliveryModel
	if err := json.NewDecoder(r.Body).Decode(&delivery); err != nil {
		return hookCommon.TransformResultModel{Error: fmt.Errorf("Failed to parse request body: %s", err)}
	}

	return transformDelivery(delivery)
}
