package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForkFlagDefaults(t *testing.T) {
	t.Parallel()

	var nilTUI *TUIOptions
	require.True(t, nilTUI.ForkFlag("anything", true))
	require.False(t, nilTUI.ForkFlag("anything", false))

	var nilCfg *Config
	require.True(t, nilCfg.ForkFlag("anything", true))
	require.True(t, (&Config{}).ForkFlag("anything", true))

	tui := &TUIOptions{Fork: map[string]bool{"on": true, "off": false}}
	require.True(t, tui.ForkFlag("on", false))
	require.False(t, tui.ForkFlag("off", true))
	require.True(t, tui.ForkFlag("missing", true))
}

func TestForkFlagKey(t *testing.T) {
	t.Parallel()
	require.Equal(t, "options.tui.fork.show_credits", ForkFlagKey("show_credits"))
}

func TestForkFlagsDecode(t *testing.T) {
	t.Parallel()

	var tui TUIOptions
	require.NoError(t, json.Unmarshal([]byte(`{"fork":{"sidebar_minimalism":false,"x":true}}`), &tui))
	require.False(t, tui.ForkFlag("sidebar_minimalism", true))
	require.True(t, tui.ForkFlag("x", false))
}

func TestForkFlagsLegacyFallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		json string
		flag string
		want bool
	}{
		{"legacy sidebar key", `{"sidebar_minimalism":false}`, "sidebar_minimalism", false},
		{"legacy credits key maps to show_credits", `{"show_hypercredits":false}`, "show_credits", false},
		{"legacy symbol key", `{"symbol_hints":false}`, "symbol_hints", false},
		{"new key wins over legacy", `{"show_hypercredits":false,"fork":{"show_credits":true}}`, "show_credits", true},
		{"non-bool legacy value is ignored", `{"symbol_hints":"no"}`, "symbol_hints", true},
		{"legacy key survives the theme migration path", `{"theme":"gruvbox-dark","sidebar_minimalism":false}`, "sidebar_minimalism", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var tui TUIOptions
			require.NoError(t, json.Unmarshal([]byte(tc.json), &tui))
			require.Equal(t, tc.want, tui.ForkFlag(tc.flag, true))
		})
	}
}

func TestForkFlagsLegacyRoundTrip(t *testing.T) {
	t.Parallel()

	// A folded legacy value travels with the config snapshot that the
	// server hands to clients.
	var tui TUIOptions
	require.NoError(t, json.Unmarshal([]byte(`{"show_hypercredits":false}`), &tui))
	data, err := json.Marshal(&tui)
	require.NoError(t, err)

	var again TUIOptions
	require.NoError(t, json.Unmarshal(data, &again))
	require.False(t, again.ForkFlag("show_credits", true))
}

func TestLegacyForkFlagsMapping(t *testing.T) {
	t.Parallel()
	require.Equal(t, map[string]string{
		"sidebar_minimalism": "sidebar_minimalism",
		"show_hypercredits":  "show_credits",
		"symbol_hints":       "symbol_hints",
	}, LegacyForkFlags())
}

func TestForkFlagPersistsThroughSetConfigField(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")
	// The legacy flat key is honored until the new key is written.
	require.NoError(t, os.WriteFile(configPath, []byte(`{"options":{"tui":{"sidebar_minimalism":false}}}`), 0o600))

	store, err := Load(t.Context(), dir, dir, false)
	require.NoError(t, err)
	require.False(t, store.config.ForkFlag("sidebar_minimalism", true))

	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	require.NoError(t, store.SetConfigField(ScopeGlobal, ForkFlagKey("sidebar_minimalism"), true))
	require.True(t, store.config.ForkFlag("sidebar_minimalism", false))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(data), `"fork"`)
}
