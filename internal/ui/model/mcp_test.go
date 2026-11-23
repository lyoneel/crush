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

// TestMCPList_DisabledColorsNameWithoutStatusWord pins the disabled
// row shape: the word "disabled" is gone and the server name carries
// the disabled status color instead.
func TestMCPList_DisabledColorsNameWithoutStatusWord(t *testing.T) {
	t.Parallel()
	styles := common.DefaultCommon(nil).Styles

	disabledSrv := []mcp.ClientInfo{
		{Name: "webhook", State: mcp.StateDisabled},
	}
	out := mcpList(styles, disabledSrv, 80, 10)

	require.NotContains(t, out, "disabled", "the disabled status word must not render")
	require.Contains(t, out, "webhook", "the server name must render")
	// The name renders exactly once, in the disabled status color
	// (96;95;107), with no normal-color sequence wrapped around it.
	require.Equal(t, 1, strings.Count(out, "\x1b[38;2;96;95;107mwebhook"),
		"the name must render in the disabled status color")
	require.NotRegexp(t, `\x1b\[38;2;96;95;107m\x1b\[38`, out,
		"no nested color sequences may wrap the name")
}

// TestMcpCountsFormat pins the LSP-style count format for the MCP
// capability counts, colored like the disabled status text:
// T<tools> P<prompts> R<resources>.
func TestMcpCountsFormat(t *testing.T) {
	styles := common.DefaultCommon(nil).Styles

	out := mcpCounts(styles, mcp.Counts{Tools: 3, Prompts: 1, Resources: 100})
	require.Contains(t, strings.Trim(out, "\x1b[m"), "38;2;96;95;107mT3")
	require.Equal(t, 3, strings.Count(out, "\x1b[38;2;96;95;107m"), "each count group carries the disabled-status color")
	// The plain text survives the styling between escape sequences.
	require.Regexp(t, `T3\x1b\[m \x1b\[38`, out)

	require.Empty(t, mcpCounts(styles, mcp.Counts{}), "zero counts must stay hidden")
	require.Equal(t, "\x1b[38;2;96;95;107mT3\x1b[m", mcpCounts(styles, mcp.Counts{Tools: 3}), "a single non-zero count renders alone, styled")
}
