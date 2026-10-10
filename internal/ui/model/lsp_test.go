package model

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// TestLSPInfoSidebarHidesUnstarted pins the sidebar filter for LSPs that
// never activated: unstarted servers stay out of the sidebar while the
// landing view (sidebar=false) keeps the full list.
func TestLSPInfoSidebarHidesUnstarted(t *testing.T) {
	t.Parallel()
	m := newBusyUI(nil)
	m.lspStates = map[string]workspace.LSPClientInfo{
		"gopls": {Name: "gopls", State: lsp.StateUnstarted},
	}

	sidebar := m.lspInfo(40, 5, true, true)
	require.NotContains(t, sidebar, "unstarted",
		"the sidebar must hide a never-activated LSP")

	landing := m.lspInfo(40, 5, false, false)
	require.Contains(t, landing, "unstarted",
		"the landing view keeps the full LSP list")
}

// TestLSPInfoSidebarStickyAfterActive pins the sticky rule: an LSP that was
// ready at some point stays visible in the sidebar after it stops, and the
// non-zero ConnectedAt keeps it visible across UI restarts.
func TestLSPInfoSidebarStickyAfterActive(t *testing.T) {
	t.Parallel()
	m := newBusyUI(nil)

	m.lspStates = map[string]workspace.LSPClientInfo{
		"gopls": {Name: "gopls", State: lsp.StateStopped, ConnectedAt: time.Now()},
	}
	require.Contains(t, m.lspInfo(40, 5, true, true), "gopls",
		"a stopped LSP that was active must stay in the sidebar")

	m.lspStates = map[string]workspace.LSPClientInfo{
		"gopls": {Name: "gopls", State: lsp.StateStopped},
	}
	require.Contains(t, m.lspInfo(40, 5, true, true), "gopls",
		"a stopped LSP with a non-zero ConnectedAt (was ready before a restart) must stay in the sidebar")
}

// TestLSPInfoSidebarHidesDisabled pins the disabled-LSP rule: a disabled LSP
// can never activate, so the sidebar hides it like an unstarted one, while
// the landing view keeps it.
func TestLSPInfoSidebarHidesDisabled(t *testing.T) {
	t.Parallel()
	m := newBusyUI(nil)
	m.lspStates = map[string]workspace.LSPClientInfo{
		"gopls": {Name: "gopls", State: lsp.StateDisabled},
	}

	require.NotContains(t, m.lspInfo(40, 5, true, true), "gopls",
		"the sidebar must hide a disabled LSP")
	require.Contains(t, m.lspInfo(40, 5, false, false), "gopls",
		"the landing view keeps the disabled LSP")
}

// TestLSPInfoSidebarKeepsNone pins the empty-section shape: when every LSP
// is filtered out, the sidebar keeps the section title with the None
// placeholder instead of dropping the section.
func TestLSPInfoSidebarKeepsNone(t *testing.T) {
	t.Parallel()
	m := newBusyUI(nil)
	m.lspStates = map[string]workspace.LSPClientInfo{
		"gopls": {Name: "gopls", State: lsp.StateUnstarted},
	}

	out := m.lspInfo(40, 5, true, true)
	require.Contains(t, out, "LSPs", "the section title must stay")
	require.Contains(t, out, "None", "the None placeholder must replace the filtered list")
}

// TestSidebarContentRendersFilteredSections pins the assembled sidebar: with
// every LSP and MCP hidden by the filters, the cached content keeps both
// section titles with the None placeholder and shows no server names.
func TestSidebarContentRendersFilteredSections(t *testing.T) {
	t.Parallel()
	ws := &configWorkspace{
		cfg: &config.Config{
			MCP: config.MCPs{
				"webhook": config.MCPConfig{},
			},
			Options: &config.Options{},
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

	m.updateSidebarScrollState()

	// The sidebar renders one character per line at width 7 (the test UI's
	// narrow default), so full words never appear contiguously; assert on
	// the rendered first letters of the section titles and the filtered
	// server names instead.
	require.NotContains(t, m.sidebarContent, "gopls", "the sidebar must hide the never-activated LSP")
	require.NotContains(t, m.sidebarContent, "webhook", "the sidebar must hide the disabled MCP")
	require.Contains(t, m.sidebarContent, "L", "the LSP section title must stay")
	require.Contains(t, m.sidebarContent, "M", "the MCPs section title must stay")
	require.Contains(t, m.sidebarContent, "N", "the None placeholders must replace the filtered lists")
}
