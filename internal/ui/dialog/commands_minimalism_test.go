package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// configWorkspaceStub satisfies the Workspace interface for the Config()
// reads the commands dialog makes while building its item list.
type configWorkspaceStub struct {
	workspace.Workspace

	cfg *config.Config
}

func (w *configWorkspaceStub) Config() *config.Config { return w.cfg }

// TestCommandsSidebarMinimalismItemWired pins the dialog wiring: the
// default command list carries the minimalism item directly below Toggle
// Sidebar with the toggle action attached. Toggle Sidebar sits in the
// session guard; the minimalism and hypercredits toggles show with or
// without an active session, behind the width guard only.
func TestCommandsSidebarMinimalismItemWired(t *testing.T) {
	t.Parallel()

	s := commonDefaultStyles()
	com := &common.Common{Styles: &s, Workspace: &configWorkspaceStub{cfg: &config.Config{}}}
	c, err := NewCommands(com, "s1", true, false, false, nil, nil)
	require.NoError(t, err)
	// The toggle items render only at a sidebar-capable width (the same
	// guard as Toggle Sidebar); setCommandItems rebuilds from windowWidth.
	c.windowWidth = sidebarCompactModeBreakpoint
	c.setCommandItems(SystemCommands)

	items := c.list.FilteredItems()
	var foundToggle, foundMinimalism, minimalismAfterToggle bool
	for _, item := range items {
		cmdItem, ok := item.(*CommandItem)
		if !ok || cmdItem == nil {
			continue
		}
		switch cmdItem.id {
		case "toggle_sidebar":
			foundToggle = true
		case "toggle_sidebar_minimalism":
			foundMinimalism = true
			minimalismAfterToggle = foundToggle
			_, isAction := cmdItem.Action().(ActionToggleSidebarMinimalism)
			require.True(t, isAction, "the item must carry the toggle action")
		}
	}
	require.True(t, foundToggle, "the Toggle Sidebar item must exist")
	require.True(t, foundMinimalism, "the Toggle Minimalist Sidebar item must exist")
	require.True(t, minimalismAfterToggle, "the minimalism item must sit directly below Toggle Sidebar")

	var foundHypercredits, hypercreditsAfterMinimalism bool
	for _, item := range items {
		cmdItem, ok := item.(*CommandItem)
		if !ok || cmdItem == nil {
			continue
		}
		if cmdItem.id == "toggle_hypercredits" {
			foundHypercredits = true
			hypercreditsAfterMinimalism = foundMinimalism
			_, isAction := cmdItem.Action().(ActionToggleHypercredits)
			require.True(t, isAction, "the item must carry the toggle action")
		}
	}
	require.True(t, foundHypercredits, "the Toggle Hypercredits item must exist")
	require.True(t, hypercreditsAfterMinimalism, "the hypercredits item must sit directly below the minimalism item")
}

// TestCommandsTogglesShowWithoutSession pins the startup-screen
// behavior: the minimalism and hypercredits toggles appear with no
// active session, while Toggle Sidebar stays in the session guard.
func TestCommandsTogglesShowWithoutSession(t *testing.T) {
	t.Parallel()

	s := commonDefaultStyles()
	com := &common.Common{Styles: &s, Workspace: &configWorkspaceStub{cfg: &config.Config{}}}
	c, err := NewCommands(com, "", false, false, false, nil, nil)
	require.NoError(t, err)
	c.windowWidth = sidebarCompactModeBreakpoint
	c.setCommandItems(SystemCommands)

	items := c.list.FilteredItems()
	var foundToggle, foundMinimalism, foundHypercredits bool
	for _, item := range items {
		cmdItem, ok := item.(*CommandItem)
		if !ok || cmdItem == nil {
			continue
		}
		switch cmdItem.id {
		case "toggle_sidebar":
			foundToggle = true
		case "toggle_sidebar_minimalism":
			foundMinimalism = true
		case "toggle_hypercredits":
			foundHypercredits = true
		}
	}
	require.False(t, foundToggle, "Toggle Sidebar must stay behind the session guard")
	require.True(t, foundMinimalism, "the minimalism toggle must show without a session")
	require.True(t, foundHypercredits, "the hypercredits toggle must show without a session")
}

// commonDefaultStyles builds the styles the commands dialog renders with.
func commonDefaultStyles() styles.Styles {
	return common.ThemeStylesFromConfig(&config.Config{})
}
