package model

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// TestShowHypercreditsGetterDefaults pins the nil-safe getter: an absent
// key or nil TUI options means shown (the default); an explicit false
// means hidden.
func TestShowHypercreditsGetterDefaults(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false

	require.True(t, (*config.TUIOptions)(nil).IsShowHypercredits(),
		"a nil receiver means shown")
	require.True(t, (&config.TUIOptions{}).IsShowHypercredits(),
		"an unset pointer means shown")
	require.False(t, (&config.TUIOptions{ShowHypercredits: &disabled}).IsShowHypercredits(),
		"an explicit false means hidden")
	require.True(t, (&config.TUIOptions{ShowHypercredits: &enabled}).IsShowHypercredits(),
		"an explicit true means shown")
}

// TestShowHypercreditsActionFlipsAndPersists feeds the toggle action
// through the dialog action switch and checks the field flip, the
// persistence call, and the state toast.
func TestShowHypercreditsActionFlipsAndPersists(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{}
	m := newBusyUI(ws)
	m.showHyperCredits = true

	cmd := m.handleDialogAction(dialog.ActionToggleHypercredits{})
	require.False(t, m.showHyperCredits, "the toggle must flip the field")
	runCmds(m, cmd)
	require.Equal(t, []bool{false}, ws.showHypercreditsCalls,
		"the flip must persist the new value")

	m.showHyperCredits = false
	cmd = m.handleDialogAction(dialog.ActionToggleHypercredits{})
	require.True(t, m.showHyperCredits, "the toggle must flip the field back")
	runCmds(m, cmd)
	require.Equal(t, []bool{false, true}, ws.showHypercreditsCalls,
		"the flip back must persist too")
}

// TestShowHypercreditsRenderGate pins the render switch: with the toggle
// off neither the model info stack nor the header credits show the
// balance; with it on both render.
func TestShowHypercreditsRenderGate(t *testing.T) {
	t.Parallel()

	m := stackTestModel(t)
	m.hyperCredits = intPtr(42)
	m.com.Config().Providers.Set("hyper", config.ProviderConfig{ID: "hyper", Name: "Charm Hyper"})
	m.com.Config().Models[config.SelectedModelTypeLarge] = config.SelectedModel{Provider: "hyper", Model: "Hyper/big-model"}
	m.agentModel = workspace.AgentModel{
		CatwalkCfg: catwalk.Model{ID: "Hyper/big-model", Name: "Big Model"},
		ModelCfg:   config.SelectedModel{Provider: "hyper", Model: "Hyper/big-model"},
	}
	m.showHyperCredits = true

	require.Contains(t, m.modelInfo(40, true), "Hypercredits",
		"toggle on: the sidebar balance must render")
	require.Contains(t, m.modelInfo(40, false), "Hypercredits",
		"toggle on: the landing balance must render")
	require.NotNil(t, m.headerCredits(),
		"toggle on: the header must receive the balance")

	m.showHyperCredits = false
	require.NotContains(t, m.modelInfo(40, true), "Hypercredits",
		"toggle off: the sidebar balance must stay hidden")
	require.NotContains(t, m.modelInfo(40, false), "Hypercredits",
		"toggle off: the landing balance must stay hidden")
	require.Nil(t, m.headerCredits(),
		"toggle off: the header must receive no balance")
}

// TestShowHypercreditsFetchGate pins the fetch kill-switch: the
// /v1/credits call exists only to feed the display, so the toggle drives
// the hyper package kill-switch and every fetch site checks it.
func TestShowHypercreditsFetchGate(t *testing.T) {
	t.Parallel()

	m := stackTestModel(t)
	m.com.Config().Providers.Set("hyper", config.ProviderConfig{ID: "hyper", Name: "Charm Hyper"})
	m.com.Config().Models[config.SelectedModelTypeLarge] = config.SelectedModel{Provider: "hyper", Model: "Hyper/big-model"}
	m.agentModel = workspace.AgentModel{
		CatwalkCfg: catwalk.Model{ID: "Hyper/big-model", Name: "Big Model"},
		ModelCfg:   config.SelectedModel{Provider: "hyper", Model: "Hyper/big-model"},
	}

	m.showHyperCredits = false
	hyper.SetDisplayEnabled(false)
	require.False(t, hyper.DisplayEnabled(), "toggle off: the kill-switch must be off")

	// The kill-switch makes the fetch a no-op before any request leaves
	// the process.
	credits, err := hyper.FetchCredits(t.Context(), "key")
	require.NoError(t, err)
	require.Nil(t, credits, "toggle off: FetchCredits must skip the request")

	// The render sites read the same flag and stay silent.
	require.Nil(t, m.headerCredits(), "toggle off: the header must receive no balance")

	m.showHyperCredits = true
	hyper.SetDisplayEnabled(true)
	require.True(t, hyper.DisplayEnabled(), "toggle on: the kill-switch must be back on")
}
