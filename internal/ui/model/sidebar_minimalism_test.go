package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// TestSidebarMinimalismGetterDefaults pins the nil-safe getter: an absent
// key or nil TUI options means enabled (the default); an explicit false
// means disabled.
func TestSidebarMinimalismGetterDefaults(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false

	require.True(t, (*config.TUIOptions)(nil).IsSidebarMinimalism(),
		"a nil receiver means enabled")
	require.True(t, (&config.TUIOptions{}).IsSidebarMinimalism(),
		"an unset pointer means enabled")
	require.False(t, (&config.TUIOptions{SidebarMinimalism: &disabled}).IsSidebarMinimalism(),
		"an explicit false means disabled")
	require.True(t, (&config.TUIOptions{SidebarMinimalism: &enabled}).IsSidebarMinimalism(),
		"an explicit true means enabled")
}

// TestSidebarMinimalismActionFlipsAndPersists feeds the toggle action
// through the dialog action switch and checks the field flip, the
// persistence call, and the state toast.
func TestSidebarMinimalismActionFlipsAndPersists(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{}
	m := newBusyUI(ws)
	m.sidebarMinimalism = true

	cmd := m.handleDialogAction(dialog.ActionToggleSidebarMinimalism{})
	require.False(t, m.sidebarMinimalism, "the toggle must flip the field")
	runCmds(m, cmd)
	require.Equal(t, []bool{false}, ws.sidebarMinimalismCalls,
		"the flip must persist the new value")

	m.sidebarMinimalism = false
	cmd = m.handleDialogAction(dialog.ActionToggleSidebarMinimalism{})
	require.True(t, m.sidebarMinimalism, "the toggle must flip the field back")
	runCmds(m, cmd)
	require.Equal(t, []bool{false, true}, ws.sidebarMinimalismCalls,
		"the flip back must persist too")
}

// TestSidebarMinimalismFilterSwitch pins the render switch: with the toggle
// off the sidebar renders the full pre-F-8 list (unstarted LSPs and
// disabled MCPs visible); with it on both stay hidden.
func TestSidebarMinimalismFilterSwitch(t *testing.T) {
	t.Parallel()

	ws := &configWorkspace{
		cfg: &config.Config{
			MCP: config.MCPs{
				"webhook": config.MCPConfig{},
			},
			Options: &config.Options{TUI: &config.TUIOptions{}},
		},
	}
	m := newBusyUI(nil)
	m.com = common.DefaultCommon(ws)
	m.lspStates = map[string]workspace.LSPClientInfo{
		"gopls": {Name: "gopls", State: lsp.StateUnstarted},
	}
	m.mcpStates = map[string]mcp.ClientInfo{
		"webhook": {Name: "webhook", State: mcp.StateDisabled},
	}

	m.sidebarMinimalism = false
	m.updateSidebarScrollState()
	// The test UI renders the sidebar at width 7 (one character per line),
	// so full words never appear contiguously; the per-letter split of
	// "gopls" and "webhook" shows they rendered (pre-F-8) instead of the
	// collapsed "None" placeholder rows.
	require.Contains(t, m.sidebarContent, "g\x1b[m      \n\x1b[38;2;133;131;146mo",
		"toggle off: the unstarted LSP must render like pre-F-8")
	require.Contains(t, m.sidebarContent, "w\x1b[m      \n\x1b[38;2;96;95;107me",
		"toggle off: the disabled MCP must render like pre-F-8")

	m.sidebarMinimalism = true
	m.updateSidebarScrollState()
	require.NotContains(t, m.sidebarContent, "g\x1b[m      \n\x1b[38;2;133;131;146mo",
		"toggle on: the unstarted LSP must stay hidden")
	require.NotContains(t, m.sidebarContent, "w\x1b[m      \n\x1b[38;2;96;95;107me",
		"toggle on: the disabled MCP must stay hidden")
}
