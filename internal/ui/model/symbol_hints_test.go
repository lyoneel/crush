package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// TestSymbolHintsStatusRenderSwitch pins the status bar render switch:
// with symbols on the quit hint reads ctl+c; with them off it reads
// ctrl+c, and turning them back on leaves no canonical residue.
func TestSymbolHintsStatusRenderSwitch(t *testing.T) {
	t.Cleanup(func() { common.SetSymbolHintsEnabled(true) })
	u := newPrismTestUI()
	u.status.helpKm = u

	render := func() string {
		return u.status.help.View(common.NewSymbolKeyMap(u.status.helpKm))
	}

	common.SetSymbolHintsEnabled(true)
	require.Contains(t, render(), "ctl+c", "symbols on: the quit hint must read ctl+c")

	common.SetSymbolHintsEnabled(false)
	view := render()
	require.Contains(t, view, "ctrl+c", "symbols off: the quit hint must read ctrl+c")
	require.NotContains(t, view, "ctl+c", "symbols off: no ctl spelling may render")

	common.SetSymbolHintsEnabled(true)
	require.Contains(t, render(), "ctl+c", "toggling back on translates again")
}
