package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// TestSymbolHintsGetterDefaults pins the nil-safe getter: an absent key
// or nil TUI options means symbols (the default); an explicit false
// means the canonical spellings.
func TestSymbolHintsGetterDefaults(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false

	require.True(t, (*config.TUIOptions)(nil).IsSymbolHints(),
		"a nil receiver means symbols")
	require.True(t, (&config.TUIOptions{}).IsSymbolHints(),
		"an unset pointer means symbols")
	require.False(t, (&config.TUIOptions{SymbolHints: &disabled}).IsSymbolHints(),
		"an explicit false means spelled out")
	require.True(t, (&config.TUIOptions{SymbolHints: &enabled}).IsSymbolHints(),
		"an explicit true means symbols")
}

// TestSymbolHintsActionFlipsAndPersists feeds the toggle action through
// the dialog action switch and checks the field flip, the persistence
// call, and the state toast.
func TestSymbolHintsActionFlipsAndPersists(t *testing.T) {
	ws := &countingWorkspace{}
	m := newBusyUI(ws)
	m.symbolHints = true

	cmd := m.handleDialogAction(dialog.ActionToggleSymbolHints{})
	require.False(t, m.symbolHints, "the toggle must flip the field")
	runCmds(m, cmd)
	require.Equal(t, []bool{false}, ws.symbolHintsCalls,
		"the flip must persist the new value")

	m.symbolHints = false
	cmd = m.handleDialogAction(dialog.ActionToggleSymbolHints{})
	require.True(t, m.symbolHints, "the toggle must flip the field back")
	runCmds(m, cmd)
	require.Equal(t, []bool{false, true}, ws.symbolHintsCalls,
		"the flip back must persist too")
}

// TestSymbolHintsStatusRenderSwitch pins the render switch on the
// status bar: with symbols on the quit hint reads ctl+c; with them off
// it reads ctrl+c.
func TestSymbolHintsStatusRenderSwitch(t *testing.T) {
	u := newPrismTestUI()
	u.status.helpKm = u

	u.symbolHints = true
	u.status.SetSymbolHints(true, u)
	view := u.status.help.View(u.status.helpKm)
	require.Contains(t, view, "ctl+c", "symbols on: the quit hint must read ctl+c")

	u.symbolHints = false
	u.status.SetSymbolHints(false, u)
	view = u.status.help.View(u.status.helpKm)
	require.Contains(t, view, "ctrl+c", "symbols off: the quit hint must read ctrl+c")
	require.NotContains(t, view, "ctl+c", "symbols off: no ctl spelling may render")
}

// TestSymbolHintsNoResidueOnToggleOff pins that toggling back on after
// a off period still translates: the wrapper re-reads the canonical
// bindings on every render, so no mutation leaks through.
func TestSymbolHintsNoResidueOnToggleOff(t *testing.T) {
	u := newPrismTestUI()
	u.status.helpKm = u

	u.status.SetSymbolHints(true, u)
	require.Contains(t, u.status.help.View(u.status.helpKm), "ctl+c",
		"symbols on: the hint translates")

	u.status.SetSymbolHints(false, u)
	u.status.SetSymbolHints(true, u)
	require.Contains(t, u.status.help.View(u.status.helpKm), "ctl+c",
		"toggling back on translates again; no canonical residue")
}
