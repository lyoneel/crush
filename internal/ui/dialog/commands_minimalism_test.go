package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// configWorkspaceStub satisfies the Workspace interface for the Config()
// reads the commands dialog makes while building its item list.
type configWorkspaceStub struct {
	workspace.Workspace

	cfg *config.Config
}

func (w *configWorkspaceStub) Config() *config.Config { return w.cfg }

// TestCommandsSymbolHintsItemWired pins the dialog wiring: the default
// command list carries the symbol-hints item behind the width guard,
// without a session guard, so it shows on the startup screen too.
func TestCommandsSymbolHintsItemWired(t *testing.T) {
	t.Parallel()

	s := commonDefaultStyles()
	com := &common.Common{Styles: &s, Workspace: &configWorkspaceStub{cfg: &config.Config{}}}
	c, err := NewCommands(com, "s1", true, false, false, false, nil, nil)
	require.NoError(t, err)
	c.windowWidth = sidebarCompactModeBreakpoint
	c.setCommandItems(SystemCommands)

	items := c.list.FilteredItems()
	var found bool
	for _, item := range items {
		cmdItem, ok := item.(*CommandItem)
		if !ok || cmdItem == nil {
			continue
		}
		if cmdItem.id == "toggle_symbol_hints" {
			found = true
			_, isAction := cmdItem.Action().(ActionToggleSymbolHints)
			require.True(t, isAction, "the item must carry the toggle action")
		}
	}
	require.True(t, found, "the Toggle Symbol Hints item must exist")
}

// commonDefaultStyles builds the styles the commands dialog renders with.
func commonDefaultStyles() styles.Styles {
	return common.ThemeStylesFromConfig(&config.Config{})
}
