package entire

import (
	"context"
	"net/http"
)

// Entire's agent catalog: the harnesses and models an agent may run, what
// they cost, and the presets a project can run as agents. Any signed-in
// caller may read it.

// AgentHarnessModels is one harness's entry in AgentsService.Models.
type AgentHarnessModels struct {
	// Harness is the agent CLI, and Name its display name.
	Harness AgentHarness `json:"harness"`
	Name    string       `json:"name"`
	// DefaultModel is the model a run uses when the agent sets none.
	DefaultModel string `json:"default_model"`
	// DefaultReasoningEffort is the effort a run uses when the agent sets
	// none; empty when the harness sets none.
	DefaultReasoningEffort ReasoningEffort `json:"default_reasoning_effort"`
	// ReasoningEfforts are every effort some model of the harness accepts,
	// lowest first.
	ReasoningEfforts []ReasoningEffort `json:"reasoning_efforts"`
	// Models are the models offered, current ones first. A hidden model is
	// not listed but is still accepted.
	Models []AgentModel `json:"models"`
}

// AgentModel is a model an agent may run.
type AgentModel struct {
	// ID is the model ID, as AgentCreateRequest.Model takes it.
	ID string `json:"id"`
	// Name is the display name, and Publisher the company that made it.
	Name      string `json:"name"`
	Publisher string `json:"publisher"`
	// Visibility is "current" for the current lineup, or "more".
	Visibility string `json:"visibility"`
	// ReasoningEfforts are the efforts an agent using the model may set,
	// lowest first; empty when it takes none.
	ReasoningEfforts []ReasoningEffort `json:"reasoning_efforts"`
}

// Models returns, for each harness an agent can use, the models it may set
// and the harness's defaults.
func (s *AgentsService) Models(ctx context.Context, reqOpts ...RequestOption) (*Response[[]AgentHarnessModels], error) {
	req := homeRequest(http.MethodGet, nil, "/agents/models")
	type envelope struct {
		Agents []AgentHarnessModels `json:"agents"`
	}
	return unwrap(ctx, s.client, req, reqOpts, func(e *envelope) []AgentHarnessModels { return e.Agents })
}

// AgentModelPrices is what the default rate card charges for agent models.
// Customers on a custom contract may pay different prices.
type AgentModelPrices struct {
	// RateCardUpdatedAt is when the prices last changed; zero when the card
	// does not say.
	RateCardUpdatedAt Timestamp `json:"rate_card_updated_at"`
	// FetchedAt is when Entire last read the rate card.
	FetchedAt Timestamp `json:"fetched_at"`
	// Agents lists each harness's models, as Models lists them.
	Agents []AgentHarnessPrices `json:"agents"`
	// OtherRates are charges not tied to a model, such as web searches.
	OtherRates []AgentOtherRate `json:"other_rates"`
}

// AgentHarnessPrices are the prices of one harness's models.
type AgentHarnessPrices struct {
	Harness AgentHarness `json:"harness"`
	// Provider is the company whose models the harness runs.
	Provider string            `json:"provider"`
	Models   []AgentModelPrice `json:"models"`
}

// AgentModelPrice is one model's price.
type AgentModelPrice struct {
	ID string `json:"id"`
	// USDPerMillionTokens is nil when the rate card lacks any of its
	// prices.
	USDPerMillionTokens *AgentTokenPrices `json:"usd_per_million_tokens"`
}

// AgentTokenPrices are prices in US dollars per million tokens, as decimal
// strings such as "0.011".
type AgentTokenPrices struct {
	Input      string `json:"input"`
	Output     string `json:"output"`
	CacheRead  string `json:"cache_read"`
	CacheWrite string `json:"cache_write"`
}

// AgentOtherRate is a charge not tied to a model.
type AgentOtherRate struct {
	Name string `json:"name"`
	// USDPerUnit is the price of one unit, as a decimal string.
	USDPerUnit string `json:"usd_per_unit"`
}

// ModelPrices returns what the default rate card charges for each listed
// model. Until Entire has read the rate card it fails with 503 Service
// Unavailable, with a Retry-After header when the prices are expected
// shortly: HTTPError.RetryAfter reads it, and a RetryPolicy honors it.
func (s *AgentsService) ModelPrices(ctx context.Context, reqOpts ...RequestOption) (*Response[AgentModelPrices], error) {
	req := homeRequest(http.MethodGet, nil, "/agents/models/prices")
	return Call[AgentModelPrices](ctx, s.client, req, reqOpts...)
}

// AgentPreset is an agent Entire defines, which a project runs by creating
// an agent with its ID as AgentCreateRequest.Preset.
type AgentPreset struct {
	// ID identifies the preset.
	ID string `json:"id"`
	// Version is the current version; it changes whenever the content
	// does.
	Version int `json:"version"`
	// SHA256 is the hash of the current content, as a run records it.
	SHA256 string `json:"sha256"`
	// Name and Description describe the preset.
	Name        string `json:"name"`
	Description string `json:"description"`
	// Agent is what every agent of the preset runs now.
	Agent AgentDefinition `json:"agent"`
}

// ListPresets returns Entire's agent presets as they are now. It is not
// paginated.
func (s *AgentsService) ListPresets(ctx context.Context, reqOpts ...RequestOption) (*Response[[]AgentPreset], error) {
	req := homeRequest(http.MethodGet, nil, "/agents/presets")
	return listCursor[AgentPreset](ctx, s.client, req, reqOpts)
}

// GetPreset returns one preset as it is now.
func (s *AgentsService) GetPreset(ctx context.Context, presetID string, reqOpts ...RequestOption) (*Response[AgentPreset], error) {
	req := homeRequest(http.MethodGet, nil, "/agents/presets/%s", presetID)
	return Call[AgentPreset](ctx, s.client, req, reqOpts...)
}
