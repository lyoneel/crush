package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

func TestMCPList_ChannelBadge(t *testing.T) {
	t.Parallel()
	styles := common.DefaultCommon(nil).Styles

	channelSrv := []mcp.ClientInfo{
		{Name: "webhook", State: mcp.StateConnected, Channel: true},
	}
	if out := mcpList(styles, channelSrv, 80, 10); !strings.Contains(out, "channel") {
		t.Errorf("expected a channel badge for an active channel server, got:\n%s", out)
	}

	plainSrv := []mcp.ClientInfo{
		{Name: "webhook", State: mcp.StateConnected, Channel: false},
	}
	if out := mcpList(styles, plainSrv, 80, 10); strings.Contains(out, "channel") {
		t.Errorf("non-channel server must not show a channel badge, got:\n%s", out)
	}
}

// TestMcpCountsFormat pins the LSP-style count format for the MCP
// capability counts: T<tools> P<prompts> R<resources>.
func TestMcpCountsFormat(t *testing.T) {
	out := mcpCounts(mcp.Counts{Tools: 3, Prompts: 1, Resources: 100})
	require.Contains(t, out, "T3 P1 R100")

	require.Empty(t, mcpCounts(mcp.Counts{}), "zero counts must stay hidden")
	require.Equal(t, "T3", mcpCounts(mcp.Counts{Tools: 3}), "a single non-zero count renders alone")
}
