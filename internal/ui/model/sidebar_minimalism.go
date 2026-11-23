package model

import (
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/fork"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/workspace"
)

// minimalistSidebar reports whether the minimalist filters apply to a
// resource section: only the sidebar renders sections (isSection), and
// only while the compact sidebar toggle is on. The landing view and the
// compact details always keep the full picture.
func (m *UI) minimalistSidebar(isSection bool) bool {
	return isSection && fork.IsSidebarMinimalism(m.com.Config())
}

// lspEverActive reports whether an LSP counts as activated for the
// sidebar's never-interacted filter. A server that left the unstarted or
// disabled state was started at least once in the app's lifetime; a
// non-zero ConnectedAt marks a server that was ready before, surviving
// UI restarts because the state round-trips through the workspace API.
func lspEverActive(info workspace.LSPClientInfo) bool {
	if info.State != lsp.StateUnstarted && info.State != lsp.StateDisabled {
		return true
	}
	return !info.ConnectedAt.IsZero()
}

// mcpHiddenInSidebar reports whether the minimalist sidebar hides an MCP
// server: a disabled server cannot do anything this session.
func mcpHiddenInSidebar(state mcp.ClientInfo) bool {
	return state.State == mcp.StateDisabled
}
