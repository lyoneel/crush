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

// TestHookItemRendersLifecycleTag pins the bracket tag per event: a
// Pre/Post/On prefix plus 2-3 uppercase letters replaces the full event
// name, and the line keeps the native shape.
func TestHookItemRendersLifecycleTag(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	tests := []struct {
		event string
		tag   string
	}{
		{"PreToolUse", "[PreTU]"},
		{"PostToolUse", "[PostTU]"},
		{"PreSession", "[PreSE]"},
		{"PreSessionSwitch", "[PreSS]"},
		{"PrePrompt", "[PrePR]"},
		{"PostPrompt", "[PostPR]"},
		{"PostResponse", "[PostRE]"},
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

// TestHookItemRendersLegacyTag pins that transcript records written
// before the event rename still render with their original two-letter
// tags.
func TestHookItemRendersLegacyTag(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	tests := []struct {
		event string
		tag   string
	}{
		{"SessionStart", "[ST]"},
		{"SessionSwitch", "[SS]"},
		{"PromptSubmitted", "[PS]"},
		{"AgentFinished", "[AF]"},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			t.Parallel()
			item := NewHookItem(&sty, message.HookContent{
				Event: tt.event,
				Hooks: []message.HookRun{{Event: tt.event, Name: "legacy.sh", Decision: "allow"}},
			})
			out := ansi.Strip(item.RawRender(100))
			require.Contains(t, out, "Hook "+tt.tag)
		})
	}
}

// TestHookItemSingleLine pins the native single-line shape: one line per
// hook run in the form `Hook [tag] name → OK`, with no separate head
// line and no blank line. This test fails on the pre-change code because
// the old output carried the head line above the body.
func TestHookItemSingleLine(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	tests := []struct {
		event string
		tag   string
	}{
		{"PrePrompt", "[PrePR]"},
		{"PromptSubmitted", "[PS]"},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			t.Parallel()
			item := NewHookItem(&sty, message.HookContent{
				Event: tt.event,
				Hooks: []message.HookRun{{Event: tt.event, Name: "prompt-pre.sh", Decision: "allow"}},
			})
			out := ansi.Strip(item.RawRender(100))
			lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
			require.Len(t, lines, 1, "a lifecycle hook run must render as exactly one line: %q", out)
			require.Equal(t, "Hook "+tt.tag+" prompt-pre.sh → OK", lines[0])
		})
	}
}

// TestHookItemDenyRendersDenied pins the deny detail text on lifecycle
// lines; it must not show OK.
func TestHookItemDenyRendersDenied(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewHookItem(&sty, message.HookContent{
		Event: "PreSessionSwitch",
		Hooks: []message.HookRun{{Event: "PreSessionSwitch", Name: "gate.sh", Decision: "deny"}},
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
		Event: "PreSessionSwitch",
		Hooks: []message.HookRun{{Event: "PreSessionSwitch", Name: "session-switch.sh", Decision: "allow"}},
	})
	for _, line := range strings.Split(ansi.Strip(item.RawRender(20)), "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 20)
	}
}

// TestHookLabelAndTagDistinctColors pins the split hook colors: the
// "Hook" word renders green and the bracket tag renders gray, each with
// its own ANSI style. This fails on the pre-fix code where both shared
// the gray HookLabel style.
func TestHookLabelAndTagDistinctColors(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewHookItem(&sty, message.HookContent{
		Event: "PreSessionSwitch",
		Hooks: []message.HookRun{{Event: "PreSessionSwitch", Name: "session-switch.sh", Decision: "allow"}},
	})
	out := item.RawRender(100)

	hookStyled := sty.Tool.HookLabel.Render("Hook")
	tagStyled := sty.Tool.HookTag.Render("[PreSS]")
	require.Contains(t, out, hookStyled, "Hook label keeps its own style")
	require.Contains(t, out, tagStyled, "bracket tag keeps its own style")
	require.NotEqual(t, sty.Tool.HookLabel, sty.Tool.HookTag,
		"label and tag styles must differ")
}
