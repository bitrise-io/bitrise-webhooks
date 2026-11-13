package entire

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twitchtv/twirp"
	"go.entire.com/entire-sdk-go/plugin"

	"github.com/bitrise-io/bitrise-webhooks/bitriseapi"
	gitprovider "github.com/bitrise-io/bitrise-webhooks/rpc/git_provider/v1beta1"
)

const (
	testCloneURL = "entire://aws-us-east-2.entire.io/et/acme/api"
	testAPIURL   = "https://aws-us-east-2.api.entire.io"
	testCommit   = "5f2c1d9a0b3e4f6a7b8c9d0e1f2a3b4c5d6e7f80"
)

type fakeGitProvider struct {
	resolveRequests []*gitprovider.ResolveWebhookTargetsRequest
	resolveResponse *gitprovider.ResolveWebhookTargetsResponse
	resolveError    error

	changeRequests []*gitprovider.ApplyInstallationChangeRequest
	changeError    error
}

func (f *fakeGitProvider) ResolveWebhookTargets(_ context.Context, request *gitprovider.ResolveWebhookTargetsRequest) (*gitprovider.ResolveWebhookTargetsResponse, error) {
	f.resolveRequests = append(f.resolveRequests, request)
	if f.resolveError != nil {
		return nil, f.resolveError
	}
	return f.resolveResponse, nil
}

func (f *fakeGitProvider) ApplyInstallationChange(_ context.Context, request *gitprovider.ApplyInstallationChangeRequest) (*gitprovider.ApplyInstallationChangeResponse, error) {
	f.changeRequests = append(f.changeRequests, request)
	if f.changeError != nil {
		return nil, f.changeError
	}
	return &gitprovider.ApplyInstallationChangeResponse{Applied: true}, nil
}

type triggerCall struct {
	url      string
	apiToken string
	params   bitriseapi.TriggerAPIParamsModel
	logOnly  bool
}

type triggerRecorder struct {
	mu    sync.Mutex
	calls []triggerCall
	err   error
	// When set, every trigger waits here, the way a slow build trigger would.
	release chan struct{}
}

func (r *triggerRecorder) trigger(_ context.Context, triggerURL *url.URL, apiToken string, params bitriseapi.TriggerAPIParamsModel, isOnlyLog bool) (bitriseapi.TriggerAPIResponseModel, bool, error) {
	if r.release != nil {
		<-r.release
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, triggerCall{url: triggerURL.String(), apiToken: apiToken, params: params, logOnly: isOnlyLog})
	if r.err != nil {
		return bitriseapi.TriggerAPIResponseModel{}, false, r.err
	}
	return bitriseapi.TriggerAPIResponseModel{Status: "ok"}, true, nil
}

func (r *triggerRecorder) recorded() []triggerCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]triggerCall(nil), r.calls...)
}

func newTestHandler(t *testing.T, gitProvider *fakeGitProvider, recorder *triggerRecorder) *Handler {
	t.Helper()
	buildTriggerURL, err := url.Parse("http://monolith.internal:3000")
	require.NoError(t, err)

	handler, err := NewHandler(Config{PluginSlug: "bitrise-test", DeliveryJWKSURL: "https://keys.example/jwks.json", BuildTriggerURL: buildTriggerURL}, gitProvider)
	require.NoError(t, err)
	handler.triggerBuild = recorder.trigger
	return handler
}

func delivery(t *testing.T, eventType string, payload any) *plugin.Delivery {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return &plugin.Delivery{
		ID:             "dlv_1",
		PluginID:       "bitrise-test",
		InstallationID: "ins_1",
		APIURL:         testAPIURL,
		Event:          plugin.Event{ID: "evt_1", Type: eventType, Payload: raw},
	}
}

func refPayload(ref, after string) map[string]any {
	return map[string]any{
		"repository": map[string]any{"id": "repo_1", "path": "/et/acme/api", "clone_url": "https://aws-us-east-2.entire.io/et/acme/api", "entire_clone_url": testCloneURL},
		"ref":        ref,
		"before":     "0000000000000000000000000000000000000000",
		"after":      after,
		"pusher":     map[string]any{"account": map[string]any{"id": "acc_1", "handle": "alice"}},
	}
}

func targets(slugs ...string) []*gitprovider.WebhookTarget {
	var list []*gitprovider.WebhookTarget
	for _, slug := range slugs {
		list = append(list, &gitprovider.WebhookTarget{AppSlug: slug, BuildTriggerToken: "token-of-" + slug})
	}
	return list
}

func Test_Handle_refEvents(t *testing.T) {
	t.Run("a branch push asks the Monolith for the projects and triggers a build for each", func(t *testing.T) {
		gitProvider := &fakeGitProvider{resolveResponse: &gitprovider.ResolveWebhookTargetsResponse{Targets: targets("app-1", "app-2"), CommitMessage: "Fix the flaky test"}}
		recorder := &triggerRecorder{}
		handler := newTestHandler(t, gitProvider, recorder)

		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, refPayload("refs/heads/main", testCommit))))
		handler.Wait()

		require.Len(t, gitProvider.resolveRequests, 1)
		request := gitProvider.resolveRequests[0]
		require.Equal(t, gitprovider.Provider_PROVIDER_ENTIRE, request.GetProvider())
		require.Equal(t, "dlv_1", request.GetDeliveryId())
		require.Equal(t, "ins_1", request.GetInstallationId())
		require.Equal(t, "repo_1", request.GetRepository().GetExternalId())
		require.Equal(t, testCommit, request.GetCommitHash())
		require.Equal(t, map[string]any{"api_url": testAPIURL, "entire_clone_url": testCloneURL}, request.GetProviderContext().AsMap())

		expectedParams := bitriseapi.TriggerAPIParamsModel{
			BuildParams: bitriseapi.BuildParamsModel{
				Branch:            "main",
				CommitHash:        testCommit,
				CommitMessage:     "Fix the flaky test",
				BaseRepositoryURL: testCloneURL,
				HeadRepositoryURL: testCloneURL,
			},
			TriggeredBy: "webhook-entire/alice",
		}
		require.ElementsMatch(t, []triggerCall{
			{url: "http://monolith.internal:3000/app/app-1/build/start.json", apiToken: "token-of-app-1", params: expectedParams},
			{url: "http://monolith.internal:3000/app/app-2/build/start.json", apiToken: "token-of-app-2", params: expectedParams},
		}, recorder.recorded())
	})

	t.Run("a tag push builds the tag", func(t *testing.T) {
		gitProvider := &fakeGitProvider{resolveResponse: &gitprovider.ResolveWebhookTargetsResponse{Targets: targets("app-1")}}
		recorder := &triggerRecorder{}
		handler := newTestHandler(t, gitProvider, recorder)

		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefCreated, refPayload("refs/tags/v1.2", testCommit))))
		handler.Wait()

		calls := recorder.recorded()
		require.Len(t, calls, 1)
		require.Equal(t, "v1.2", calls[0].params.BuildParams.Tag)
		require.Empty(t, calls[0].params.BuildParams.Branch)
		require.Empty(t, calls[0].params.BuildParams.CommitMessage)
	})

	t.Run("the delivery is answered while the build triggers are still running", func(t *testing.T) {
		gitProvider := &fakeGitProvider{resolveResponse: &gitprovider.ResolveWebhookTargetsResponse{Targets: targets("app-1")}}
		recorder := &triggerRecorder{release: make(chan struct{})}
		handler := newTestHandler(t, gitProvider, recorder)

		handled := make(chan error, 1)
		go func() {
			handled <- handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, refPayload("refs/heads/main", testCommit)))
		}()

		select {
		case err := <-handled:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Handle waited for the build trigger")
		}
		require.Empty(t, recorder.recorded())

		close(recorder.release)
		handler.Wait()
		require.Len(t, recorder.recorded(), 1)
	})

	t.Run("a failed build trigger does not fail the delivery or the other projects", func(t *testing.T) {
		gitProvider := &fakeGitProvider{resolveResponse: &gitprovider.ResolveWebhookTargetsResponse{Targets: targets("app-1", "app-2")}}
		recorder := &triggerRecorder{err: errors.New("the Monolith is restarting")}
		handler := newTestHandler(t, gitProvider, recorder)

		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, refPayload("refs/heads/main", testCommit))))
		handler.Wait()

		require.Len(t, recorder.recorded(), 2)
	})

	t.Run("a skip ci commit message triggers nothing", func(t *testing.T) {
		gitProvider := &fakeGitProvider{resolveResponse: &gitprovider.ResolveWebhookTargetsResponse{Targets: targets("app-1"), CommitMessage: "Update the README [skip ci]"}}
		recorder := &triggerRecorder{}
		handler := newTestHandler(t, gitProvider, recorder)

		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, refPayload("refs/heads/main", testCommit))))
		handler.Wait()

		require.Empty(t, recorder.recorded())
	})

	t.Run("a repository without a project triggers nothing", func(t *testing.T) {
		gitProvider := &fakeGitProvider{resolveResponse: &gitprovider.ResolveWebhookTargetsResponse{}}
		recorder := &triggerRecorder{}
		handler := newTestHandler(t, gitProvider, recorder)

		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, refPayload("refs/heads/main", testCommit))))
		handler.Wait()

		require.Len(t, gitProvider.resolveRequests, 1)
		require.Empty(t, recorder.recorded())
	})

	for name, payload := range map[string]map[string]any{
		"a deleted ref":                  refPayload("refs/heads/main", "0000000000000000000000000000000000000000"),
		"a ref event without a commit":   refPayload("refs/heads/main", ""),
		"a ref that is no branch or tag": refPayload("refs/notes/commits", testCommit),
		"a branch ref without a name":    refPayload("refs/heads/", testCommit),
	} {
		t.Run(name+" is acknowledged without asking the Monolith", func(t *testing.T) {
			gitProvider := &fakeGitProvider{}
			recorder := &triggerRecorder{}
			handler := newTestHandler(t, gitProvider, recorder)

			require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, payload)))
			handler.Wait()

			require.Empty(t, gitProvider.resolveRequests)
			require.Empty(t, recorder.recorded())
		})
	}

	t.Run("a lookup the Monolith cannot answer now is an error, so that Entire redelivers", func(t *testing.T) {
		for name, lookupError := range map[string]error{
			"unavailable":     twirp.NewError(twirp.Unavailable, "try again"),
			"internal":        twirp.InternalError("boom"),
			"transport error": errors.New("dial tcp: connection refused"),
			"timeout":         context.DeadlineExceeded,
		} {
			t.Run(name, func(t *testing.T) {
				gitProvider := &fakeGitProvider{resolveError: lookupError}
				recorder := &triggerRecorder{}
				handler := newTestHandler(t, gitProvider, recorder)

				require.Error(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, refPayload("refs/heads/main", testCommit))))
				handler.Wait()

				require.Empty(t, recorder.recorded())
			})
		}
	})

	t.Run("a lookup the Monolith refuses is acknowledged, because a redelivery would be refused too", func(t *testing.T) {
		gitProvider := &fakeGitProvider{resolveError: twirp.InvalidArgumentError("installation_id", "is required")}
		recorder := &triggerRecorder{}
		handler := newTestHandler(t, gitProvider, recorder)

		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, refPayload("refs/heads/main", testCommit))))
		handler.Wait()

		require.Empty(t, recorder.recorded())
	})
}

func Test_Handle_installationEvents(t *testing.T) {
	installationPayload := func(installation map[string]any) map[string]any {
		return map[string]any{"installation": installation}
	}

	t.Run("a suspension is passed on with who suspended it and Entire's revision", func(t *testing.T) {
		gitProvider := &fakeGitProvider{}
		handler := newTestHandler(t, gitProvider, &triggerRecorder{})

		payload := installationPayload(map[string]any{"project_id": "prj_1", "revision": 7, "suspended_by": "customer"})
		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventInstallationSuspended, payload)))

		require.Len(t, gitProvider.changeRequests, 1)
		request := gitProvider.changeRequests[0]
		require.Equal(t, gitprovider.Provider_PROVIDER_ENTIRE, request.GetProvider())
		require.Equal(t, "dlv_1", request.GetDeliveryId())
		require.Equal(t, "ins_1", request.GetInstallationId())
		require.Equal(t, gitprovider.InstallationChange_INSTALLATION_CHANGE_SUSPENDED, request.GetChange())
		require.Equal(t, int64(7), request.GetSequence())
		require.Equal(t, map[string]any{"event_type": "installation.suspended", "suspended_by": "customer"}, request.GetDetails().AsMap())
	})

	t.Run("an update carries the metadata the event has", func(t *testing.T) {
		gitProvider := &fakeGitProvider{}
		handler := newTestHandler(t, gitProvider, &triggerRecorder{})

		payload := installationPayload(map[string]any{
			"project_id": "prj_1", "project": map[string]any{"id": "prj_1", "name": "acme", "forge": "entire"},
			"revision": 3, "repository_selection": "selected", "manage_url": "https://entire.io/acme/settings/plugins/bitrise",
		})
		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventInstallationUpdated, payload)))

		require.Len(t, gitProvider.changeRequests, 1)
		request := gitProvider.changeRequests[0]
		require.Equal(t, gitprovider.InstallationChange_INSTALLATION_CHANGE_UPDATED, request.GetChange())
		require.Equal(t, map[string]any{
			"event_type": "installation.updated", "repository_selection": "selected", "project_name": "acme", "project_forge": "entire",
			"manage_url": "https://entire.io/acme/settings/plugins/bitrise",
		}, request.GetDetails().AsMap())
	})

	for eventType, expected := range map[string]gitprovider.InstallationChange{
		plugin.EventInstallationResumed:             gitprovider.InstallationChange_INSTALLATION_CHANGE_RESUMED,
		plugin.EventInstallationDeleted:             gitprovider.InstallationChange_INSTALLATION_CHANGE_DELETED,
		plugin.EventInstallationCreated:             gitprovider.InstallationChange_INSTALLATION_CHANGE_UPDATED,
		plugin.EventInstallationRepositoriesAdded:   gitprovider.InstallationChange_INSTALLATION_CHANGE_UPDATED,
		plugin.EventInstallationRepositoriesRemoved: gitprovider.InstallationChange_INSTALLATION_CHANGE_UPDATED,
	} {
		t.Run(eventType+" is passed on as "+expected.String(), func(t *testing.T) {
			gitProvider := &fakeGitProvider{}
			handler := newTestHandler(t, gitProvider, &triggerRecorder{})

			require.NoError(t, handler.Handle(context.Background(), delivery(t, eventType, installationPayload(map[string]any{"revision": 2}))))

			require.Len(t, gitProvider.changeRequests, 1)
			require.Equal(t, expected, gitProvider.changeRequests[0].GetChange())
			require.Equal(t, int64(2), gitProvider.changeRequests[0].GetSequence())
		})
	}

	t.Run("an installation no workspace has connected is acknowledged", func(t *testing.T) {
		gitProvider := &fakeGitProvider{changeError: twirp.NotFoundError("Entire installation ins_1 is not connected to a workspace.")}
		handler := newTestHandler(t, gitProvider, &triggerRecorder{})

		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventInstallationDeleted, installationPayload(map[string]any{"revision": 2}))))
	})

	t.Run("a change the Monolith cannot apply now is an error, so that Entire redelivers", func(t *testing.T) {
		gitProvider := &fakeGitProvider{changeError: twirp.NewError(twirp.Unavailable, "the installation is being updated, please retry")}
		handler := newTestHandler(t, gitProvider, &triggerRecorder{})

		require.Error(t, handler.Handle(context.Background(), delivery(t, plugin.EventInstallationSuspended, installationPayload(map[string]any{"revision": 2}))))
	})
}

func Test_Handle_otherEvents(t *testing.T) {
	for _, eventType := range []string{plugin.EventRefDeleted, plugin.EventTrailOpened, plugin.EventTrailUpdated, plugin.EventTrailMerged, "something.new"} {
		t.Run(eventType+" is acknowledged and goes no further", func(t *testing.T) {
			gitProvider := &fakeGitProvider{}
			recorder := &triggerRecorder{}
			handler := newTestHandler(t, gitProvider, recorder)

			require.NoError(t, handler.Handle(context.Background(), delivery(t, eventType, refPayload("refs/heads/main", testCommit))))
			handler.Wait()

			require.Empty(t, gitProvider.resolveRequests)
			require.Empty(t, gitProvider.changeRequests)
			require.Empty(t, recorder.recorded())
		})
	}
}

func Test_Handle_triggerDestination(t *testing.T) {
	t.Run("SendRequestToURL replaces the build trigger URL, as for the other providers", func(t *testing.T) {
		gitProvider := &fakeGitProvider{resolveResponse: &gitprovider.ResolveWebhookTargetsResponse{Targets: targets("app-1")}}
		recorder := &triggerRecorder{}
		handler := newTestHandler(t, gitProvider, recorder)
		sendRequestTo, err := url.Parse("http://localhost:9999/debug")
		require.NoError(t, err)
		handler.cfg.SendRequestToURL = sendRequestTo
		handler.cfg.LogOnlyMode = true

		require.NoError(t, handler.Handle(context.Background(), delivery(t, plugin.EventRefUpdated, refPayload("refs/heads/main", testCommit))))
		handler.Wait()

		calls := recorder.recorded()
		require.Len(t, calls, 1)
		require.Equal(t, "http://localhost:9999/debug", calls[0].url)
		require.True(t, calls[0].logOnly)
	})
}

func Test_NewHandler(t *testing.T) {
	_, err := NewHandler(Config{DeliveryJWKSURL: "https://keys.example/jwks.json"}, &fakeGitProvider{})
	require.Error(t, err)

	_, err = NewHandler(Config{PluginSlug: "bitrise-test"}, &fakeGitProvider{})
	require.Error(t, err)

	_, err = NewHandler(Config{PluginSlug: "bitrise-test", DeliveryJWKSURL: "https://keys.example/jwks.json"}, nil)
	require.Error(t, err)

	handler, err := NewHandler(Config{PluginSlug: "bitrise-test", DeliveryJWKSURL: "https://keys.example/jwks.json"}, &fakeGitProvider{})
	require.NoError(t, err)

	// The SDK accepts the placeholder signing identity, so the route can be built without the plugin's key.
	httpHandler, err := handler.HTTPHandler()
	require.NoError(t, err)
	require.NotNil(t, httpHandler)
}
