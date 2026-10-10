package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestDiscoverModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [
				{"id": "model-a", "object": "model", "owned_by": "org"},
				{"id": "model-b", "object": "model", "owned_by": "org"}
			]
		}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "model-a", models[0].ID)
	require.Equal(t, "model-b", models[1].ID)
}

func TestDiscoverModels_ExistingModelsWin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "model-a", "object": "model"},
				{"id": "model-b", "object": "model"}
			]
		}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		ExistingModels: []catwalk.Model{
			{ID: "model-a", Name: "My Custom Name", ContextWindow: 200000, CanReason: true},
		},
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 2)

	require.Equal(t, "model-a", models[0].ID)
	require.Equal(t, "My Custom Name", models[0].Name)
	require.Equal(t, int64(200000), models[0].ContextWindow)
	require.True(t, models[0].CanReason)

	require.Equal(t, "model-b", models[1].ID)
	require.Equal(t, "model-b", models[1].Name)
}

func TestDiscoverModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.Error(t, err)
	require.Nil(t, models)
}

func TestDiscoverModels_ExtraHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "custom-value", r.Header.Get("X-Custom"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "m1", "object": "model"}]}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		ExtraHeaders: map[string]string{
			"X-Custom": "custom-value",
		},
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
}

type mockResolver struct{}

func (m *mockResolver) ResolveValue(val string) (string, error) { return val, nil }

type envResolver struct {
	env map[string]string
}

func (e *envResolver) ResolveValue(val string) (string, error) {
	if v, ok := e.env[val]; ok {
		return v, nil
	}
	return val, nil
}

func TestDiscoverModels_ResolvesShellVariables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer resolved-key", r.Header.Get("Authorization"))
		require.Equal(t, "resolved-header", r.Header.Get("X-Custom"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "m1", "object": "model"}]}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "$MY_API_KEY",
		ExtraHeaders: map[string]string{
			"X-Custom": "$MY_HEADER",
		},
	}

	resolver := &envResolver{env: map[string]string{
		"$MY_API_KEY": "resolved-key",
		"$MY_HEADER":  "resolved-header",
	}}

	models, err := DiscoverModels(context.Background(), cfg, resolver)
	require.NoError(t, err)
	require.Len(t, models, 1)
}

func TestDiscoverModels_SkipsEmptyExtraHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("X-Empty"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "m1", "object": "model"}]}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		ExtraHeaders: map[string]string{
			"X-Empty": "$UNSET_VAR",
		},
	}

	resolver := &envResolver{env: map[string]string{
		"$UNSET_VAR": "",
	}}

	models, err := DiscoverModels(context.Background(), cfg, resolver)
	require.NoError(t, err)
	require.Len(t, models, 1)
}

func TestDiscoverModels_NoAuthWhenNoAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "m1", "object": "model"}]}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "",
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
}

func TestStripV1Suffix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:8000/v1", "http://localhost:8000"},
		{"http://localhost:8000/v1/", "http://localhost:8000"},
		{"http://localhost:8000", "http://localhost:8000"},
		{"http://localhost:8000/", "http://localhost:8000"},
		{"http://localhost:8000/api/v1", "http://localhost:8000/api"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripV1Suffix(tt.input)
		require.Equal(t, tt.want, got, "stripV1Suffix(%q)", tt.input)
	}
}

// TestDiscoverModelsMetadata pins the metadata mapping: a gateway that
// attaches a metadata block (Hyper-style) has its context window, max
// tokens, pricing, and reasoning capability carried into the catalog.
// A bare entry still maps with zero values.
func TestDiscoverModelsMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "Hyper/glm-5.3-flash",
					"object": "model",
					"owned_by": "Hyper",
					"metadata": {
						"display_name": "glm-5.3-flash",
						"context_window": 1048575,
						"max_output_tokens": 943717,
						"capabilities": {"reasoning": true, "function_calling": true},
						"pricing": {"currency": "USD", "input_per_mtok": 0.15, "output_per_mtok": 0.5, "cached_input_per_mtok": 0.03}
					}
				},
				{"id": "bare-model", "object": "model"}
			]
		}`))
	}))
	defer server.Close()

	models, err := DiscoverModels(context.Background(), Config{ID: "t", BaseURL: server.URL + "/v1", APIKey: "k"}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 2)

	rich := models[0]
	require.Equal(t, "Hyper/glm-5.3-flash", rich.ID)
	require.Equal(t, "glm-5.3-flash", rich.Name)
	require.Equal(t, int64(1048575), rich.ContextWindow)
	require.Equal(t, int64(943717), rich.DefaultMaxTokens)
	require.True(t, rich.CanReason)
	require.Equal(t, 0.15, rich.CostPer1MIn)
	require.Equal(t, 0.5, rich.CostPer1MOut)
	require.Equal(t, 0.03, rich.CostPer1MInCached)

	bare := models[1]
	require.Equal(t, "bare-model", bare.ID)
	require.Equal(t, "bare-model", bare.Name)
	require.Equal(t, int64(0), bare.ContextWindow)
	require.False(t, bare.CanReason)
}
