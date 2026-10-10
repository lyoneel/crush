package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// configWorkspace stubs the workspace Config call so mcpInfo and the
// sidebar assembly can iterate the configured servers without a real
// workspace.
type configWorkspace struct {
	workspace.Workspace

	cfg        *config.Config
	workingDir string
}

func (w *configWorkspace) Config() *config.Config { return w.cfg }

func (w *configWorkspace) WorkingDir() string { return w.workingDir }

// TestMCPInfoSidebarHidesDisabled pins the sidebar filter for disabled MCP
// servers: the sidebar keeps the section title with the None placeholder
// while the landing view (sidebar=false) keeps the server row.
func TestMCPInfoSidebarHidesDisabled(t *testing.T) {
	t.Parallel()
	ws := &configWorkspace{
		cfg: &config.Config{
			MCP: config.MCPs{
				"webhook": config.MCPConfig{},
			},
			Options: &config.Options{},
		},
	}
	m := &UI{com: common.DefaultCommon(ws)}
	m.mcpStates = map[string]mcp.ClientInfo{
		"webhook": {Name: "webhook", State: mcp.StateDisabled},
	}

	sidebar := m.mcpInfo(40, 5, true, true)
	require.NotContains(t, sidebar, "webhook", "the sidebar must hide a disabled MCP")
	require.Contains(t, sidebar, "MCPs", "the section title must stay")
	require.Contains(t, sidebar, "None", "the None placeholder must replace the filtered list")

	landing := m.mcpInfo(40, 5, false, false)
	require.Contains(t, landing, "webhook", "the landing view keeps the disabled MCP")
}
