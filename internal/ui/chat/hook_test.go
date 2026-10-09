package chat

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestHookItemRendersLifecycleTag pins the lifecycle head: a two-letter
// bracket tag replaces the full event name, and the body keeps the
// native PreToolUse line shape.
func TestHookItemRendersLifecycleTag(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	tests := []struct {
		event string
		tag   string
	}{
		{"SessionSwitch", "[SS]"},
		{"PromptSubmitted", "[PS]"},
		{"AgentFinished", "[AF]"},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			t.Parallel()
			item := NewHookItem(&sty, message.HookContent{
				Event: tt.event,
				Hooks: []message.HookRun{{Event: tt.event, Name: "session-switch.sh", Decision: "allow"}},
			})
			out := ansi.Strip(item.RawRender(100))
			require.Contains(t, out, "Hook "+tt.tag)
			require.Contains(t, out, "session-switch.sh")
			require.Contains(t, out, "OK")
		})
	}
}

// TestHookItemDenyRendersDenied pins the deny detail text on lifecycle
// lines; it must not show OK.
func TestHookItemDenyRendersDenied(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewHookItem(&sty, message.HookContent{
		Event: "SessionSwitch",
		Hooks: []message.HookRun{{Event: "SessionSwitch", Name: "gate.sh", Decision: "deny"}},
	})
	out := ansi.Strip(item.RawRender(100))
	require.Contains(t, out, "Denied")
	require.NotContains(t, out, "OK")
}

// TestHookItemNarrowWidth pins that no rendered line overflows the
// requested width.
func TestHookItemNarrowWidth(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewHookItem(&sty, message.HookContent{
		Event: "SessionSwitch",
		Hooks: []message.HookRun{{Event: "SessionSwitch", Name: "session-switch.sh", Decision: "allow"}},
	})
	for _, line := range strings.Split(ansi.Strip(item.RawRender(20)), "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 20)
	}
}
