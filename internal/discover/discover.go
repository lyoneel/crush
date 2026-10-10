package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

// httpClient is shared across all discovery and enrichment calls. It
// has a reasonable timeout so individual requests cannot block forever
// even if the caller forgets to set a context deadline.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// stripV1Suffix removes a trailing /v1 from a base URL. Enricher
// endpoints (e.g. Ollama's /api/show, LM Studio's /api/v1/models) are
// served at the server root, not under the OpenAI-compatible /v1
// prefix. Since provider configs typically include /v1 in the base URL
// for chat completions, enrichers must strip it before constructing
// their own request paths.
func stripV1Suffix(baseURL string) string {
	return strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")
}

// doRequest builds and executes an authenticated HTTP request using the
// shared client. It resolves variable references in the base URL, API
// key, and extra headers via the provided Resolver. The path is joined
// to the base URL with proper slash handling.
func doRequest(ctx context.Context, method, baseURL, path, apiKey string, extraHeaders map[string]string, resolver Resolver, body any) (*http.Response, error) {
	resolvedBase, _ := resolver.ResolveValue(baseURL)
	resolvedKey, _ := resolver.ResolveValue(apiKey)

	url := strings.TrimRight(resolvedBase, "/") + "/" + strings.TrimLeft(path, "/")

	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, reqBody)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, err
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if resolvedKey != "" {
		req.Header.Set("Authorization", "Bearer "+resolvedKey)
	}
	for k, v := range extraHeaders {
		resolved, err := resolver.ResolveValue(v)
		if err != nil || resolved == "" {
			continue
		}
		req.Header.Set(k, resolved)
	}

	return httpClient.Do(req)
}

// Config holds the provider configuration needed for model discovery.
type Config struct {
	ID           string
	BaseURL      string
	APIKey       string
	ExtraHeaders map[string]string
	// Existing models from config — IDs present in this list are skipped
	// during discovery (user-specified models win).
	ExistingModels []catwalk.Model
}

// Resolver resolves variable references (e.g. $ENV_VAR) in config values.
type Resolver interface {
	ResolveValue(val string) (string, error)
}

type modelsResponse struct {
	Data []discoveredModel `json:"data"`
}

// discoveredModel mirrors the OpenAI-compatible /models entry, plus the
// optional `metadata` block several gateways attach (Hyper, NIM, and
// friends) carrying the context window, output cap, capabilities, and
// pricing metadata that /models alone would lose.
type discoveredModel struct {
	ID       string          `json:"id"`
	Object   string          `json:"object"`
	Created  int64           `json:"created"`
	OwnedBy  string          `json:"owned_by"`
	Metadata *discoveredMeta `json:"metadata,omitempty"`
}

// discoveredMeta is the shared metadata block. Fields the gateway
// omits stay zero and the model keeps falling back to sensible
// defaults.
type discoveredMeta struct {
	DisplayName     string             `json:"display_name,omitempty"`
	ContextWindow   int64              `json:"context_window,omitempty"`
	MaxOutputTokens int64              `json:"max_output_tokens,omitempty"`
	Capabilities    discoveredCaps     `json:"capabilities,omitempty"`
	Pricing         *discoveredPricing `json:"pricing,omitempty"`
}

type discoveredCaps struct {
	FunctionCalling bool `json:"function_calling,omitempty"`
	Reasoning       bool `json:"reasoning,omitempty"`
	ToolChoice      bool `json:"tool_choice,omitempty"`
}

type discoveredPricing struct {
	Currency           string  `json:"currency,omitempty"`
	InputPerMtok       float64 `json:"input_per_mtok,omitempty"`
	OutputPerMtok      float64 `json:"output_per_mtok,omitempty"`
	CachedInputPerMtok float64 `json:"cached_input_per_mtok,omitempty"`
}

// DiscoverModels fetches available models from the provider's /models endpoint.
// It uses the provided context for cancellation and timeout; callers should set
// a deadline (e.g. context.WithTimeout) to avoid blocking indefinitely.
// Models whose IDs already appear in cfg.ExistingModels are skipped —
// user-specified models take precedence.
func DiscoverModels(ctx context.Context, cfg Config, resolver Resolver) ([]catwalk.Model, error) {
	resp, err := doRequest(ctx, http.MethodGet, cfg.BaseURL, "/models", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	if err != nil {
		return nil, fmt.Errorf("discover models for provider %s: %w", cfg.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover models for provider %s: %s", cfg.ID, resp.Status)
	}

	var modelsResp modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("discover models for provider %s: %w", cfg.ID, err)
	}

	// Build set of existing model IDs to skip.
	existing := make(map[string]struct{}, len(cfg.ExistingModels))
	for _, m := range cfg.ExistingModels {
		existing[m.ID] = struct{}{}
	}

	// Start with user-specified models.
	result := make([]catwalk.Model, len(cfg.ExistingModels))
	copy(result, cfg.ExistingModels)

	// Append discovered models not already in the list, mapping the
	// gateway metadata block onto the catwalk fields when present.
	for _, e := range modelsResp.Data {
		if _, ok := existing[e.ID]; ok {
			continue
		}
		m := catwalk.Model{
			ID:   e.ID,
			Name: e.ID,
		}
		if e.Metadata != nil {
			md := e.Metadata
			if md.DisplayName != "" {
				m.Name = md.DisplayName
			}
			m.ContextWindow = md.ContextWindow
			m.DefaultMaxTokens = md.MaxOutputTokens
			if md.Capabilities.Reasoning {
				m.CanReason = true
				m.ReasoningLevels = []string{"high"}
				m.DefaultReasoningEffort = "high"
			}
			if md.Pricing != nil {
				m.CostPer1MIn = md.Pricing.InputPerMtok
				m.CostPer1MOut = md.Pricing.OutputPerMtok
				m.CostPer1MInCached = md.Pricing.CachedInputPerMtok
			}
		}
		result = append(result, m)
	}

	return result, nil
}
