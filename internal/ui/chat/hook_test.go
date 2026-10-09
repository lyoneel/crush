package chat

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/hooks"
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
// hook run in the form `✓ Hook [tag] name → OK`, with the completion
// glyph marking a lifecycle line, no separate head line, and no blank
// line. This test fails on the pre-change code because the old output
// carried the head line above the body.
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
			require.Equal(t, "✓ Hook "+tt.tag+" prompt-pre.sh → OK", lines[0])
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

// TestHookFailureStatuses pins that a hook run which did not complete
// renders its execution status instead of OK: Timeout for a timed-out
// run and the exit code for any other failure, both in the yellow
// failure style.
func TestHookFailureStatuses(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{"timeout", hooks.StatusTimeout, "Timeout"},
		{"exit code", "exit 5", "exit 5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			item := NewHookItem(&sty, message.HookContent{
				Event: "PrePrompt",
				Hooks: []message.HookRun{{Event: "PrePrompt", Name: "f.sh", Decision: "none", Status: tt.status}},
			})
			raw := item.RawRender(100)
			out := ansi.Strip(raw)
			require.Contains(t, out, tt.expected)
			require.NotContains(t, out, "OK")
			require.Contains(t, raw, sty.Tool.HookFailed.Render(tt.expected))
		})
	}
}

// TestHookCompletionGlyph pins the lifecycle completion marker: a green
// check before the label on a completed run and a yellow cross on a
// failed run. The glyph separates lifecycle completion notices from the
// inline PreToolUse indicator, which has no glyph.
func TestHookCompletionGlyph(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	t.Run("success renders green check", func(t *testing.T) {
		t.Parallel()
		item := NewHookItem(&sty, message.HookContent{
			Event: "PrePrompt",
			Hooks: []message.HookRun{{Event: "PrePrompt", Name: "f.sh", Decision: "none"}},
		})
		raw := item.RawRender(100)
		require.Contains(t, raw, sty.Tool.JobIconSuccess.SetString(styles.ToolSuccess).Render())
		require.NotContains(t, raw, styles.ToolError)
	})

	t.Run("failure renders yellow cross", func(t *testing.T) {
		t.Parallel()
		item := NewHookItem(&sty, message.HookContent{
			Event: "PrePrompt",
			Hooks: []message.HookRun{{Event: "PrePrompt", Name: "f.sh", Decision: "none", Status: "exit 5"}},
		})
		raw := item.RawRender(100)
		require.Contains(t, raw, sty.Tool.HookFailed.SetString(styles.ToolError).Render())
		require.NotContains(t, raw, styles.ToolSuccess)
	})

	t.Run("tool indicator has no glyph", func(t *testing.T) {
		t.Parallel()
		meta := &hooks.HookMetadata{
			Event:     hooks.EventPreToolUse,
			HookCount: 1,
			Hooks:     []hooks.HookInfo{{Name: "rtk-rewrite.sh", Decision: "allow"}},
		}
		out := ansi.Strip(toolOutputHookIndicator(&sty, hookMetaJSON(meta, nil), 100))
		require.Contains(t, out, "Hook [PreTU]")
		require.False(t, strings.Contains(out, styles.ToolSuccess+" Hook"),
			"the inline PreToolUse line must not carry a completion glyph")
	})
}
