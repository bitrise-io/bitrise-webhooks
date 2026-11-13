// Package entire receives the webhook deliveries of the Bitrise plugin on Entire (https://entire.io).
//
// Entire has one webhook URL per plugin, so a delivery names an installation and a repository but no
// Bitrise project. That is why this is not a hookCommon.Provider behind /h/{service-id}/{app-slug}/{api-token}:
// the handler has its own route, verifies the delivery with Entire's SDK, and asks the Monolith which
// projects the repository belongs to before it triggers their builds.
package entire

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/twitchtv/twirp"
	"go.entire.com/entire-sdk-go/plugin"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/bitrise-io/api-utils/logging"
	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	gitprovider "github.com/bitrise-io/bitrise-webhooks/rpc/git_provider/v1beta1"
	hookCommon "github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

const (
	// ProviderID ...
	ProviderID = "entire"

	tagRefPrefix             = "refs/tags/"
	installationEventPrefix  = "installation."
	pluginIssuerPrefix       = "urn:entire:plugin:"
	maxParallelBuildTriggers = 8

	// Entire waits ten seconds for the answer to a delivery, and the Monolith's reply is only a part of that.
	monolithTimeout = 5 * time.Second
)

// GitProviderService is the part of the Monolith's generated GitProviderService client this handler calls.
type GitProviderService interface {
	ResolveWebhookTargets(context.Context, *gitprovider.ResolveWebhookTargetsRequest) (*gitprovider.ResolveWebhookTargetsResponse, error)
	ApplyInstallationChange(context.Context, *gitprovider.ApplyInstallationChangeRequest) (*gitprovider.ApplyInstallationChangeResponse, error)
}

// TriggerBuildFunc is the signature of bitriseapi.TriggerBuild.
type TriggerBuildFunc func(ctx context.Context, url *url.URL, apiToken string, params bitriseapi.TriggerAPIParamsModel, isOnlyLog bool) (bitriseapi.TriggerAPIResponseModel, bool, error)

// Config ...
type Config struct {
	// PluginSlug is the slug Entire registered the Bitrise plugin with; deliveries are addressed to it.
	PluginSlug string
	// DeliveryJWKSURL is the JWKS URL Entire handed out for delivery JWTs.
	DeliveryJWKSURL string
	// BuildTriggerURL, SendRequestToURL and LogOnlyMode mean what they mean for every other provider.
	BuildTriggerURL  *url.URL
	SendRequestToURL *url.URL
	LogOnlyMode      bool
}

// Handler ...
type Handler struct {
	cfg          Config
	gitProvider  GitProviderService
	triggerBuild TriggerBuildFunc
	// Build triggers that are still running after their delivery was answered.
	running sync.WaitGroup
}

// NewHandler ...
func NewHandler(cfg Config, gitProvider GitProviderService) (*Handler, error) {
	if cfg.PluginSlug == "" || cfg.DeliveryJWKSURL == "" {
		return nil, errors.New("entire: the plugin slug and the delivery JWKS URL are required")
	}
	if gitProvider == nil {
		return nil, errors.New("entire: a GitProviderService client is required")
	}
	return &Handler{cfg: cfg, gitProvider: gitProvider, triggerBuild: bitriseapi.TriggerBuild}, nil
}

// NewGitProviderClient returns the client of the Monolith's GitProviderService at baseURL. Its requests
// carry the trace of the delivery, so one trace covers a delivery in both services.
func NewGitProviderClient(baseURL *url.URL) GitProviderService {
	httpClient := &http.Client{Timeout: monolithTimeout, Transport: tracePropagatingTransport{}}
	return gitprovider.NewGitProviderServiceProtobufClient(strings.TrimRight(baseURL.String(), "/"), httpClient)
}

// HTTPHandler returns the handler of the webhook URL. Entire's SDK verifies each delivery (the JWT
// against the delivery JWKS, its audience, times and the digest of the raw body) and answers on its own
// for the ones it rejects; Handle only sees verified deliveries.
func (h *Handler) HTTPHandler() (http.Handler, error) {
	// The SDK asks for the plugin's signing identity even though verifying a delivery signs nothing.
	// The real key stays with the service that mints plugin JWTs, so this one is never published or used.
	_, unusedKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("entire: generate placeholder key: %w", err)
	}

	app, err := plugin.New(plugin.Config{
		Slug:            h.cfg.PluginSlug,
		Issuer:          pluginIssuerPrefix + h.cfg.PluginSlug,
		Signer:          unusedKey,
		Permissions:     plugin.Permissions{Contents: plugin.Read},
		DeliveryJWKSURL: h.cfg.DeliveryJWKSURL,
	})
	if err != nil {
		return nil, fmt.Errorf("entire: %w", err)
	}
	return app.WebhookHandler(h.Handle), nil
}

// Handle processes one verified delivery. Returning an error makes the SDK answer 500, which Entire
// retries, so it is returned only when a retry can help.
func (h *Handler) Handle(ctx context.Context, delivery *plugin.Delivery) error {
	ctx = logging.NewContext(ctx,
		zap.String("entire_delivery_id", delivery.ID),
		zap.String("entire_installation_id", delivery.InstallationID),
		zap.String("entire_event_type", delivery.Event.Type),
	)
	if span, ok := tracer.SpanFromContext(ctx); ok {
		span.SetTag("entire.delivery_id", delivery.ID)
		span.SetTag("entire.installation_id", delivery.InstallationID)
		span.SetTag("entire.event_type", delivery.Event.Type)
	}

	switch eventType := delivery.Event.Type; {
	case eventType == plugin.EventRefCreated || eventType == plugin.EventRefUpdated:
		return h.handleRefChange(ctx, delivery)
	case strings.HasPrefix(eventType, installationEventPrefix):
		return h.handleInstallationChange(ctx, delivery)
	default:
		// Entire may add event types to the same URL, and asks for a 2xx on the ones a plugin does not handle.
		logging.WithContext(ctx).Info("Entire delivery acknowledged without processing")
		return nil
	}
}

// Wait blocks until the build triggers of the deliveries handled so far have returned.
func (h *Handler) Wait() {
	h.running.Wait()
}

func (h *Handler) handleRefChange(ctx context.Context, delivery *plugin.Delivery) error {
	logger := logging.WithContext(ctx)

	change, err := delivery.Event.RefChange()
	if err != nil {
		logger.Error("Entire ref event could not be decoded, acknowledging it", zap.Error(err))
		return nil
	}

	triggerParams, skipReason := triggerParamsOf(change)
	if skipReason != "" {
		logger.Info("Entire ref event starts no build", zap.String("skip_reason", skipReason), zap.String("ref", change.Ref))
		return nil
	}

	resolved, err := h.resolveTargets(ctx, delivery, change)
	if err != nil {
		return h.monolithError(ctx, "ResolveWebhookTargets", err)
	}
	if len(resolved.GetTargets()) == 0 {
		logger.Info("Entire ref event starts no build", zap.String("skip_reason", "no_project"), zap.String("ref", change.Ref))
		return nil
	}

	triggerParams.BuildParams.CommitMessage = resolved.GetCommitMessage()
	if hookCommon.ContainsSkipInstruction(triggerParams.BuildParams.CommitMessage) {
		logger.Info("Entire ref event starts no build", zap.String("skip_reason", "skip_ci"), zap.String("ref", change.Ref),
			zap.String("commit_hash", change.After))
		return nil
	}

	h.triggerBuilds(ctx, resolved.GetTargets(), triggerParams)
	return nil
}

func (h *Handler) resolveTargets(ctx context.Context, delivery *plugin.Delivery, change *plugin.RefChange) (*gitprovider.ResolveWebhookTargetsResponse, error) {
	providerContext, err := structpb.NewStruct(map[string]any{
		"api_url":          delivery.APIURL,
		"entire_clone_url": change.Repository.EntireCloneURL,
	})
	if err != nil {
		return nil, twirp.InvalidArgumentError("provider_context", err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, monolithTimeout)
	defer cancel()

	return h.gitProvider.ResolveWebhookTargets(ctx, &gitprovider.ResolveWebhookTargetsRequest{
		Provider:        gitprovider.Provider_PROVIDER_ENTIRE,
		DeliveryId:      delivery.ID,
		InstallationId:  delivery.InstallationID,
		Repository:      &gitprovider.RepositoryRef{Ref: &gitprovider.RepositoryRef_ExternalId{ExternalId: change.Repository.ID}},
		CommitHash:      change.After,
		ProviderContext: providerContext,
	})
}

// triggerBuilds starts one build per target and does not wait for them, as the GitLab provider does not
// wait for its trigger: Entire is answered first, because the build trigger of one project alone can
// outlast the ten seconds Entire waits. Nothing retries a call that fails; its outcome is only logged.
func (h *Handler) triggerBuilds(ctx context.Context, targets []*gitprovider.WebhookTarget, triggerParams bitriseapi.TriggerAPIParamsModel) {
	// The delivery's context is cancelled as soon as Entire has its answer.
	ctx = context.WithoutCancel(ctx)

	h.running.Add(1)
	go func() {
		defer h.running.Done()

		slots := make(chan struct{}, maxParallelBuildTriggers)
		var triggers sync.WaitGroup
		for _, target := range targets {
			slots <- struct{}{}
			triggers.Add(1)
			go func() {
				defer triggers.Done()
				defer func() { <-slots }()
				h.triggerBuildOf(ctx, target, triggerParams)
			}()
		}
		triggers.Wait()
	}()
}

func (h *Handler) triggerBuildOf(ctx context.Context, target *gitprovider.WebhookTarget, triggerParams bitriseapi.TriggerAPIParamsModel) {
	logger := logging.WithContext(ctx).With(zap.String("app_slug", target.GetAppSlug()))

	triggerURL := h.cfg.SendRequestToURL
	if triggerURL == nil {
		if h.cfg.BuildTriggerURL == nil {
			logger.Error("Entire build trigger not sent: no build trigger URL is configured")
			return
		}
		buildTriggerURL, err := bitriseapi.BuildTriggerURL(h.cfg.BuildTriggerURL, target.GetAppSlug())
		if err != nil {
			logger.Error("Entire build trigger not sent", zap.Error(err))
			return
		}
		triggerURL = buildTriggerURL
	}

	response, accepted, err := h.triggerBuild(ctx, triggerURL, target.GetBuildTriggerToken(), triggerParams, h.cfg.LogOnlyMode)
	switch {
	case err != nil:
		logger.Error("Entire build trigger failed", zap.Error(err))
	case !accepted:
		logger.Warn("Entire build trigger was rejected", zap.String("status", response.Status), zap.String("message", response.Message))
	default:
		logger.Info("Entire build trigger was accepted", zap.String("status", response.Status), zap.String("message", response.Message))
	}
}

func (h *Handler) handleInstallationChange(ctx context.Context, delivery *plugin.Delivery) error {
	logger := logging.WithContext(ctx)

	change, err := delivery.Event.InstallationChange()
	if err != nil {
		logger.Error("Entire installation event could not be decoded, acknowledging it", zap.Error(err))
		return nil
	}

	details, err := structpb.NewStruct(installationDetailsOf(delivery.Event.Type, change.Installation))
	if err != nil {
		logger.Error("Entire installation event could not be passed on, acknowledging it", zap.Error(err))
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, monolithTimeout)
	defer cancel()

	response, err := h.gitProvider.ApplyInstallationChange(ctx, &gitprovider.ApplyInstallationChangeRequest{
		Provider:       gitprovider.Provider_PROVIDER_ENTIRE,
		DeliveryId:     delivery.ID,
		InstallationId: delivery.InstallationID,
		Change:         installationChangeOf(delivery.Event.Type),
		Sequence:       change.Installation.Revision,
		Details:        details,
	})
	if err != nil {
		return h.monolithError(ctx, "ApplyInstallationChange", err)
	}

	logger.Info("Entire installation change passed on", zap.Int64("revision", change.Installation.Revision), zap.Bool("applied", response.GetApplied()))
	return nil
}

// monolithError decides between a redelivery and an acknowledgement. A request the Monolith refuses as
// unusable, such as one for an installation it does not know, stays unusable however often Entire sends it.
func (h *Handler) monolithError(ctx context.Context, call string, err error) error {
	var twirpError twirp.Error
	if errors.As(err, &twirpError) {
		switch twirpError.Code() {
		case twirp.InvalidArgument, twirp.NotFound, twirp.Malformed, twirp.FailedPrecondition:
			logging.WithContext(ctx).Warn("The Monolith refused an Entire delivery, acknowledging it",
				zap.String("call", call), zap.String("code", string(twirpError.Code())), zap.String("message", twirpError.Msg()))
			return nil
		}
	}
	return fmt.Errorf("entire: %s: %w", call, err)
}

// triggerParamsOf turns a ref change into the build params every other provider sends. The second value
// is empty for a change that should build, and says why otherwise.
func triggerParamsOf(change *plugin.RefChange) (bitriseapi.TriggerAPIParamsModel, string) {
	if change.After == "" {
		return bitriseapi.TriggerAPIParamsModel{}, "no_commit"
	}
	if plugin.IsZeroSHA(change.After) {
		return bitriseapi.TriggerAPIParamsModel{}, "ref_deleted"
	}

	// The same entire:// URL the project was registered with, for both sides: the Monolith reads a
	// difference between the two as a build from a fork.
	repositoryURL := change.Repository.EntireCloneURL
	buildParams := bitriseapi.BuildParamsModel{
		CommitHash:        change.After,
		BaseRepositoryURL: repositoryURL,
		HeadRepositoryURL: repositoryURL,
	}

	if branch, isBranch := change.Branch(); isBranch && branch != "" {
		buildParams.Branch = branch
	} else if tag, isTag := strings.CutPrefix(change.Ref, tagRefPrefix); isTag && tag != "" {
		buildParams.Tag = tag
	} else {
		return bitriseapi.TriggerAPIParamsModel{}, "unsupported_ref"
	}

	return bitriseapi.TriggerAPIParamsModel{
		BuildParams: buildParams,
		TriggeredBy: hookCommon.GenerateTriggeredBy(ProviderID, change.Pusher.Account.Handle),
	}, ""
}

func installationChangeOf(eventType string) gitprovider.InstallationChange {
	switch eventType {
	case plugin.EventInstallationSuspended:
		return gitprovider.InstallationChange_INSTALLATION_CHANGE_SUSPENDED
	case plugin.EventInstallationResumed:
		return gitprovider.InstallationChange_INSTALLATION_CHANGE_RESUMED
	case plugin.EventInstallationDeleted:
		return gitprovider.InstallationChange_INSTALLATION_CHANGE_DELETED
	default:
		// Every other installation event still moves the installation's revision forward.
		return gitprovider.InstallationChange_INSTALLATION_CHANGE_UPDATED
	}
}

func installationDetailsOf(eventType string, installation plugin.InstallationState) map[string]any {
	details := map[string]any{"event_type": eventType}
	if installation.SuspendedBy != "" {
		details["suspended_by"] = string(installation.SuspendedBy)
	}
	if installation.RepositorySelection != "" {
		details["repository_selection"] = installation.RepositorySelection
	}
	if installation.ManageURL != "" {
		details["manage_url"] = installation.ManageURL
	}
	if installation.Project != nil {
		if installation.Project.Name != "" {
			details["project_name"] = installation.Project.Name
		}
		if installation.Project.Forge != "" {
			details["project_forge"] = string(installation.Project.Forge)
		}
	}
	return details
}

type tracePropagatingTransport struct{}

// RoundTrip ...
func (tracePropagatingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if span, ok := tracer.SpanFromContext(request.Context()); ok {
		request = request.Clone(request.Context())
		if err := tracer.Inject(span.Context(), tracer.HTTPHeadersCarrier(request.Header)); err != nil {
			logging.WithContext(request.Context()).Warn("Failed to propagate tracing context", zap.Error(err))
		}
	}
	return http.DefaultTransport.RoundTrip(request)
}
