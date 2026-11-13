package entire

import (
	"context"
	"errors"
	"iter"
	"net/http"
)

// ServiceAccountsService manages service accounts: non-human accounts that
// belong to a project, the access they are granted, the OIDC bindings that
// let workloads sign in as them, and their secrets. Every method calls Core.
type ServiceAccountsService service

// ServiceAccount is a non-human account that belongs to a project. Names are
// unique within the project.
type ServiceAccount struct {
	// ID identifies the service account, such as "svc_01JABC...". Pass it
	// to the ServiceAccountsService methods as accountID, and to
	// WithWorkloadIdentity.
	ID string `json:"accountId"`
	// Name is the service account's name, unique within its project.
	Name string `json:"name"`
	// ProjectID identifies the project that owns the service account.
	ProjectID string `json:"project_id"`
	// OrgID identifies the organization that owns the project, and is
	// empty when an account owns it.
	//
	// Deprecated: service accounts belong to projects. Core will stop
	// sending this field; read the project's owner with ProjectsService.Get.
	OrgID string `json:"orgId"`
	// Status is the service account's state, such as "active".
	Status string `json:"status"`
	// SystemManaged reports an account that Entire manages. It cannot be
	// deleted, and cannot be given grants, bindings, or secrets.
	SystemManaged bool `json:"systemManaged"`
	// CreatedAt is when the service account was created.
	CreatedAt Timestamp `json:"createdAt"`
	// Grants lists the platform-level relations the service account holds.
	// Only List sets it.
	Grants []string `json:"grants"`
}

// ServiceAccountCreateRequest creates a service account.
type ServiceAccountCreateRequest struct {
	// ProjectID identifies the project that will own the service account.
	// Required.
	ProjectID string `json:"project_id"`
	// Name is the service account's name, unique within the project.
	// Required.
	Name string `json:"name"`
}

// ServiceAccountListOptions selects the service accounts that
// ServiceAccountsService.List returns. Set exactly one of ProjectID and
// OrgID.
type ServiceAccountListOptions struct {
	// ProjectID lists one project's service accounts.
	ProjectID string `url:"project_id"`
	// OrgID lists the service accounts of every project an organization
	// owns. Prefer ProjectID: service accounts belong to projects.
	OrgID string `url:"orgId"`
	ListOptions
}

// ServiceAccountResourceType is the kind of resource a service account
// grant is on.
type ServiceAccountResourceType string

// The resource types of a service account grant.
const (
	ServiceAccountResourceRepo    ServiceAccountResourceType = "repo"
	ServiceAccountResourceProject ServiceAccountResourceType = "project"
)

// ServiceAccountRole is the role a service account grant gives.
type ServiceAccountRole string

// The roles a service account grant can give.
const (
	ServiceAccountRoleReader ServiceAccountRole = "reader"
	ServiceAccountRoleWriter ServiceAccountRole = "writer"
	ServiceAccountRoleAdmin  ServiceAccountRole = "admin"
)

// ServiceAccountGrant is a service account's role on a repository or
// project.
type ServiceAccountGrant struct {
	// ResourceType is the kind of resource: ServiceAccountResourceRepo or
	// ServiceAccountResourceProject.
	ResourceType ServiceAccountResourceType `json:"resourceType"`
	// ResourceID identifies the repository or project.
	ResourceID string `json:"resourceId"`
	// ResourceName is the resource's name, when Core knows it. Only
	// ListGrants sets it.
	ResourceName string `json:"resourceName,omitempty"`
	// Role is the role the grant gives.
	Role ServiceAccountRole `json:"role"`
}

// ServiceAccountGrantRequest gives a service account a role on a repository
// or project.
type ServiceAccountGrantRequest struct {
	// ResourceType is the kind of resource. Required.
	ResourceType ServiceAccountResourceType `json:"resourceType"`
	// ResourceID identifies the repository or project. Required.
	ResourceID string `json:"resourceId"`
	// Role is the role to give. Required.
	Role ServiceAccountRole `json:"role"`
}

// ServiceAccountBinding lets workloads that hold a token from an OIDC
// identity provider sign in as a service account.
type ServiceAccountBinding struct {
	// ID identifies the binding, such as "eib_01JABC...".
	ID string `json:"id"`
	// AccountID identifies the service account.
	AccountID string `json:"accountId"`
	// ProviderID identifies the OIDC provider, as OIDCProvider.ID.
	ProviderID string `json:"providerId"`
	// AttributeFilter maps the provider's attribute names to the values a
	// token must carry, such as {"repository": "acme/web"}. An attribute
	// that is not exact takes a glob, such as "deploy-*".
	AttributeFilter map[string]string `json:"attributeFilter"`
	// CreatedAt is when the binding was created.
	CreatedAt Timestamp `json:"createdAt"`
}

// ServiceAccountBindingCreateRequest creates an OIDC binding.
type ServiceAccountBindingCreateRequest struct {
	// ProviderID identifies the OIDC provider, as OIDCProvider.ID.
	// Required.
	ProviderID string `json:"providerId"`
	// AttributeFilter maps the provider's attribute names to the values a
	// token must carry. It must pin every attribute the provider marks
	// Required, and at least one of its RequireOneOf. Required.
	AttributeFilter map[string]string `json:"attributeFilter"`
}

// ServiceAccountSecret describes a service account secret. It never holds
// the secret itself.
type ServiceAccountSecret struct {
	// ID identifies the secret; pass it to DeleteSecret.
	ID string `json:"id"`
	// AccountID identifies the service account.
	AccountID string `json:"account_id"`
	// CreatedBy identifies the account that created the secret. It is empty
	// once that account is erased.
	CreatedBy string `json:"created_by"`
	// CreatedAt is when the secret was created.
	CreatedAt Timestamp `json:"created_at"`
	// ExpiresAt is when the secret stops working.
	ExpiresAt Timestamp `json:"expires_at"`
	// LastUsedAt is when the secret was last exchanged, to the minute, and
	// zero if it never was.
	LastUsedAt Timestamp `json:"last_used_at"`
	// LastUsedUserAgent is the User-Agent of the last exchange. The client
	// chose it, so treat it as a hint.
	LastUsedUserAgent string `json:"last_used_user_agent"`
}

// NewServiceAccountSecret is a secret that was just created. Its Secret is
// shown only once.
type NewServiceAccountSecret struct {
	ServiceAccountSecret
	// Secret is the secret, ent_svc_…. Store it now: Core never returns it
	// again. Pass it to NewClient to act as the service account.
	Secret string `json:"secret"`
}

// ServiceAccountSecretCreateRequest creates a service account secret.
type ServiceAccountSecretCreateRequest struct {
	// ExpiresInDays is the secret's lifetime in days, from 1 to 365. Zero
	// leaves it to Core, which defaults to 90.
	ExpiresInDays int `json:"expires_in_days,omitempty"`
}

// OIDCProvider is an OIDC identity provider that a service account binding
// can trust, such as GitHub Actions.
type OIDCProvider struct {
	// ID identifies the provider; pass it as
	// ServiceAccountBindingCreateRequest.ProviderID.
	ID string `json:"id"`
	// Issuer is the provider's issuer URL, or a template with one variable
	// path segment, such as "https://oidc.vercel.com/{team}".
	Issuer string `json:"issuer"`
	// DisplayName is the provider's name for people.
	//
	// Deprecated: use Name for the slug and Label for the human label.
	DisplayName string `json:"displayName"`
	// Description describes the provider.
	Description string `json:"description"`
	// Name is the provider's stable slug, such as "github-actions".
	Name string `json:"name"`
	// Label is the provider's name for people, such as "GitHub Actions".
	Label string `json:"label"`
	// DocsURL links to the provider's OIDC setup guide.
	DocsURL string `json:"docs_url"`
	// SelfServe reports whether a project manager can bind the provider.
	// CreateBinding refuses other providers with 403.
	SelfServe bool `json:"self_serve"`
	// Attributes are the attributes a binding's filter can pin.
	Attributes []OIDCProviderAttribute `json:"attributes"`
	// RequireOneOf lists attributes of which a filter must pin at least
	// one.
	RequireOneOf []string `json:"require_one_of"`
}

// OIDCProviderAttribute is an attribute that a binding's filter can pin.
type OIDCProviderAttribute struct {
	// Name is the attribute's key in a binding's AttributeFilter.
	Name string `json:"name"`
	// Label is the attribute's name for people.
	Label string `json:"label"`
	// Description describes the attribute.
	Description string `json:"description"`
	// Required reports that every filter must pin the attribute.
	Required bool `json:"required"`
	// Exact reports that the pinned value must not contain '*'.
	Exact bool `json:"exact"`
	// Pattern is an RE2 expression the pinned value must match, if any.
	Pattern string `json:"pattern"`
}

// Problem types that ServiceAccountsService methods fail with, in
// HTTPError.Type.
const (
	// ProblemServiceAccountNameInUse: Create named a service account that
	// already exists in the project (409).
	ProblemServiceAccountNameInUse = "https://entire.com/errors/service-account/name-in-use"
	// ProblemServiceAccountQuotaReached: the project holds as many service
	// accounts as it may (409).
	ProblemServiceAccountQuotaReached = "https://entire.com/errors/service-account/quota-reached"
	// ProblemServiceAccountBindingExists: the service account already has a
	// binding for the provider (409).
	ProblemServiceAccountBindingExists = "https://entire.com/errors/service-account/binding-exists"
	// ProblemServiceAccountBindingNotSelfServe: the provider is not
	// SelfServe, so only Entire's operators can bind it (403).
	ProblemServiceAccountBindingNotSelfServe = "https://entire.com/errors/service-account/binding-provider-not-self-serve"
	// ProblemServiceAccountSecretLimit: the service account holds as many
	// active secrets as it may; delete one first (409).
	ProblemServiceAccountSecretLimit = "https://entire.com/errors/service-account/secret-limit-reached"
)

// Create creates a service account in a project. The caller must be able to
// manage the project; a project that does not exist or that the caller
// cannot manage fails with 403. A name already in use in the project fails
// with 409 and ProblemServiceAccountNameInUse.
func (s *ServiceAccountsService) Create(ctx context.Context, body ServiceAccountCreateRequest, reqOpts ...RequestOption) (*Response[ServiceAccount], error) {
	return Call[ServiceAccount](ctx, s.client, coreRequest(http.MethodPost, body, "/service-accounts"), reqOpts...)
}

// List returns one page of the service accounts in a project, or in the
// projects of an organization. The caller must be able to view the project
// or organization.
func (s *ServiceAccountsService) List(ctx context.Context, opts *ServiceAccountListOptions, reqOpts ...RequestOption) (*Response[[]ServiceAccount], error) {
	req := coreRequest(http.MethodGet, nil, "/service-accounts")
	o := Deref(opts)
	if (o.ProjectID == "") == (o.OrgID == "") {
		req.invalid = errors.New("entire: list service accounts by exactly one of ProjectID and OrgID")
	}
	addQuery(req.Query, &o)
	o.ListOptions.add(req.Query, "pageSize", "pageToken")
	type envelope struct {
		ServiceAccounts []ServiceAccount `json:"serviceAccounts"`
		NextPageToken   string           `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[ServiceAccount] {
		return page[ServiceAccount]{items: e.ServiceAccounts, next: e.NextPageToken}
	})
}

// ListAll iterates over every service account that List returns, page by
// page.
func (s *ServiceAccountsService) ListAll(ctx context.Context, opts *ServiceAccountListOptions, reqOpts ...RequestOption) iter.Seq2[ServiceAccount, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]ServiceAccount], error) {
		p := o
		p.PageToken = token
		return s.List(ctx, &p, reqOpts...)
	})
}

// Get returns a service account. The caller must be able to view its
// project.
func (s *ServiceAccountsService) Get(ctx context.Context, accountID string, reqOpts ...RequestOption) (*Response[ServiceAccount], error) {
	return Call[ServiceAccount](ctx, s.client, coreRequest(http.MethodGet, nil, "/service-accounts/%s", accountID), reqOpts...)
}

// Delete deletes a service account and revokes its grants; its secrets and
// bindings stop working. The caller must be able to manage its project.
// Access tokens already issued to it stay valid until they expire.
func (s *ServiceAccountsService) Delete(ctx context.Context, accountID string, reqOpts ...RequestOption) (*Response[struct{}], error) {
	return Call[struct{}](ctx, s.client, coreRequest(http.MethodDelete, nil, "/service-accounts/%s", accountID), reqOpts...)
}

// Grant gives a service account a role on a repository or project, which
// must belong to the service account's project; another one fails with 422.
// The caller must be able to manage the resource.
func (s *ServiceAccountsService) Grant(ctx context.Context, accountID string, body ServiceAccountGrantRequest, reqOpts ...RequestOption) (*Response[struct{}], error) {
	return Call[struct{}](ctx, s.client, coreRequest(http.MethodPost, body, "/service-accounts/%s/grants", accountID), reqOpts...)
}

// ListGrants returns one page of a service account's grants. It holds only
// grants on resources the caller can manage, so a page can be shorter than
// PageSize, or empty, while Response.Next is set; ListGrantsAll follows the
// pages.
func (s *ServiceAccountsService) ListGrants(ctx context.Context, accountID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]ServiceAccountGrant], error) {
	req := coreRequest(http.MethodGet, nil, "/service-accounts/%s/grants", accountID)
	Deref(opts).add(req.Query, "pageSize", "pageToken")
	type envelope struct {
		Grants        []ServiceAccountGrant `json:"grants"`
		NextPageToken string                `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[ServiceAccountGrant] {
		return page[ServiceAccountGrant]{items: e.Grants, next: e.NextPageToken}
	})
}

// ListGrantsAll iterates over every grant that ListGrants returns, page by
// page.
func (s *ServiceAccountsService) ListGrantsAll(ctx context.Context, accountID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[ServiceAccountGrant, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]ServiceAccountGrant], error) {
		return s.ListGrants(ctx, accountID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// Revoke removes a service account's role on a repository or project. The
// caller must be able to manage the resource. A grant that does not exist
// fails with 404.
func (s *ServiceAccountsService) Revoke(ctx context.Context, accountID string, resourceType ServiceAccountResourceType, resourceID string, reqOpts ...RequestOption) (*Response[struct{}], error) {
	req := coreRequest(http.MethodDelete, nil, "/service-accounts/%s/grants/%s/%s", accountID, string(resourceType), resourceID)
	return Call[struct{}](ctx, s.client, req, reqOpts...)
}

// CreateBinding lets workloads that hold a token from an OIDC provider sign
// in as the service account; see WithWorkloadIdentity. The caller must be
// able to manage the service account's project. A provider that is not
// SelfServe fails with 403 and ProblemServiceAccountBindingNotSelfServe, and
// so does a caller whose token was exchanged from a service account secret.
// A second binding for the same provider fails with 409.
func (s *ServiceAccountsService) CreateBinding(ctx context.Context, accountID string, body ServiceAccountBindingCreateRequest, reqOpts ...RequestOption) (*Response[ServiceAccountBinding], error) {
	req := coreRequest(http.MethodPost, body, "/service-accounts/%s/bindings", accountID)
	return Call[ServiceAccountBinding](ctx, s.client, req, reqOpts...)
}

// ListBindings returns one page of a service account's OIDC bindings. The
// caller must be able to view its project.
func (s *ServiceAccountsService) ListBindings(ctx context.Context, accountID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]ServiceAccountBinding], error) {
	req := coreRequest(http.MethodGet, nil, "/service-accounts/%s/bindings", accountID)
	Deref(opts).add(req.Query, "pageSize", "pageToken")
	type envelope struct {
		Bindings      []ServiceAccountBinding `json:"bindings"`
		NextPageToken string                  `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[ServiceAccountBinding] {
		return page[ServiceAccountBinding]{items: e.Bindings, next: e.NextPageToken}
	})
}

// ListBindingsAll iterates over every binding that ListBindings returns,
// page by page.
func (s *ServiceAccountsService) ListBindingsAll(ctx context.Context, accountID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[ServiceAccountBinding, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]ServiceAccountBinding], error) {
		return s.ListBindings(ctx, accountID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// DeleteBinding removes an OIDC binding. The caller must be able to manage
// the service account's project. Deleting a binding that does not exist
// succeeds; one that belongs to another service account fails with 404.
// Access tokens already issued through the binding stay valid until they
// expire.
func (s *ServiceAccountsService) DeleteBinding(ctx context.Context, accountID, bindingID string, reqOpts ...RequestOption) (*Response[struct{}], error) {
	req := coreRequest(http.MethodDelete, nil, "/service-accounts/%s/bindings/%s", accountID, bindingID)
	req.Idempotent = true
	return Call[struct{}](ctx, s.client, req, reqOpts...)
}

// CreateSecret creates a secret for the service account and returns it,
// once. Pass the secret to NewClient to act as the service account. Only a
// person's own login session may create one: a service account's token, or
// one exchanged from a secret, fails with 403. The caller must be able to
// manage the service account's project. A service account at its limit of
// active secrets fails with 409 and ProblemServiceAccountSecretLimit.
//
// Secrets live in the Core of the service account's home region. Another
// Core answers 421, and the client sends the request again to the home Core.
func (s *ServiceAccountsService) CreateSecret(ctx context.Context, accountID string, body ServiceAccountSecretCreateRequest, reqOpts ...RequestOption) (*Response[NewServiceAccountSecret], error) {
	req := coreRequest(http.MethodPost, body, "/service-accounts/%s/secrets", accountID)
	return Call[NewServiceAccountSecret](ctx, s.client, req, reqOpts...)
}

// ListSecrets returns one page of a service account's secrets, newest
// first, including recently expired ones. It never returns a secret itself.
// The caller must be able to view the service account's project. Like
// CreateSecret, it follows Core's 421 to the home Core.
func (s *ServiceAccountsService) ListSecrets(ctx context.Context, accountID string, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]ServiceAccountSecret], error) {
	req := coreRequest(http.MethodGet, nil, "/service-accounts/%s/secrets", accountID)
	Deref(opts).add(req.Query, "per_page", "cursor")
	type envelope struct {
		Secrets    []ServiceAccountSecret `json:"secrets"`
		NextCursor string                 `json:"next_cursor"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[ServiceAccountSecret] {
		return page[ServiceAccountSecret]{items: e.Secrets, next: e.NextCursor}
	})
}

// ListSecretsAll iterates over every secret that ListSecrets returns, page
// by page.
func (s *ServiceAccountsService) ListSecretsAll(ctx context.Context, accountID string, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[ServiceAccountSecret, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]ServiceAccountSecret], error) {
		return s.ListSecrets(ctx, accountID, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}

// DeleteSecret revokes a service account secret. The caller must be able to
// manage the service account's project. Deleting a secret that does not
// exist succeeds. Access tokens already exchanged from the secret stay
// valid until they expire, at most 10 minutes. Like CreateSecret, it follows
// Core's 421 to the home Core.
func (s *ServiceAccountsService) DeleteSecret(ctx context.Context, accountID, secretID string, reqOpts ...RequestOption) (*Response[struct{}], error) {
	req := coreRequest(http.MethodDelete, nil, "/service-accounts/%s/secrets/%s", accountID, secretID)
	req.Idempotent = true
	return Call[struct{}](ctx, s.client, req, reqOpts...)
}

// ListOIDCProviders returns one page of the OIDC identity providers that a
// service account binding can trust. Every caller sees every provider; a
// project manager can bind only those with SelfServe set.
func (s *ServiceAccountsService) ListOIDCProviders(ctx context.Context, opts *ListOptions, reqOpts ...RequestOption) (*Response[[]OIDCProvider], error) {
	req := coreRequest(http.MethodGet, nil, "/oidc-providers")
	Deref(opts).add(req.Query, "pageSize", "pageToken")
	type envelope struct {
		Providers     []OIDCProvider `json:"providers"`
		NextPageToken string         `json:"nextPageToken"`
	}
	return list(ctx, s.client, req, reqOpts, func(e *envelope) page[OIDCProvider] {
		return page[OIDCProvider]{items: e.Providers, next: e.NextPageToken}
	})
}

// ListOIDCProvidersAll iterates over every provider that ListOIDCProviders
// returns, page by page.
func (s *ServiceAccountsService) ListOIDCProvidersAll(ctx context.Context, opts *ListOptions, reqOpts ...RequestOption) iter.Seq2[OIDCProvider, error] {
	o := snapshot(opts)
	return all(o.PageToken, func(token string) (*Response[[]OIDCProvider], error) {
		return s.ListOIDCProviders(ctx, &ListOptions{PageSize: o.PageSize, PageToken: token}, reqOpts...)
	})
}
