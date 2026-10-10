package chat

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/list"
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
			item := NewHookItem(&sty, "msg-1", message.HookContent{
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
			item := NewHookItem(&sty, "msg-1", message.HookContent{
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
			item := NewHookItem(&sty, "msg-1", message.HookContent{
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
	item := NewHookItem(&sty, "msg-1", message.HookContent{
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
	item := NewHookItem(&sty, "msg-1", message.HookContent{
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
	item := NewHookItem(&sty, "msg-1", message.HookContent{
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
			item := NewHookItem(&sty, "msg-1", message.HookContent{
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
		item := NewHookItem(&sty, "msg-1", message.HookContent{
			Event: "PrePrompt",
			Hooks: []message.HookRun{{Event: "PrePrompt", Name: "f.sh", Decision: "none"}},
		})
		raw := item.RawRender(100)
		require.Contains(t, raw, sty.Tool.JobIconSuccess.SetString(styles.ToolSuccess).Render())
		require.NotContains(t, raw, styles.ToolError)
	})

	t.Run("failure renders yellow cross", func(t *testing.T) {
		t.Parallel()
		item := NewHookItem(&sty, "msg-1", message.HookContent{
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

// TestHookPendingLine pins the running state of an async hook: a green
// pending dot before the label and no status detail yet, matching the
// bash tool's running indicator.
func TestHookPendingLine(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewHookItem(&sty, "msg-1", message.HookContent{
		Event: "PrePrompt",
		Hooks: []message.HookRun{{Event: "PrePrompt", Name: "slow-notify.sh", Decision: "none", Status: hooks.StatusRunning}},
	})
	raw := item.RawRender(100)
	out := ansi.Strip(raw)

	require.Contains(t, out, "● Hook [PrePR] slow-notify.sh")
	require.NotContains(t, out, "OK")
}

// TestHookItemSetContentPinsUpdate pins the in-place update path: the
// same item id serves the pending and the completed record, and
// SetContent swaps the rendered lines and bumps the version so the
// list cache invalidates.
func TestHookItemSetContentPinsUpdate(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	hookItem := NewHookItem(&sty, "msg-1", message.HookContent{
		Event: "PrePrompt",
		Hooks: []message.HookRun{{Event: "PrePrompt", Name: "slow-notify.sh", Decision: "none", Status: hooks.StatusRunning}},
	})
	item, ok := hookItem.(*HookItem)
	require.True(t, ok)
	before := item.Version()
	item.SetContent(message.HookContent{
		Event: "PrePrompt",
		Hooks: []message.HookRun{{Event: "PrePrompt", Name: "slow-notify.sh", Decision: "none"}},
	})
	after := item.Version()

	require.Greater(t, after, before, "SetContent must bump the version")
	out := ansi.Strip(item.RawRender(100))
	require.Contains(t, out, "✓ Hook [PrePR] slow-notify.sh → OK")
	require.NotContains(t, out, "●")
}

// TestHookLabelColumnAlignment pins that the "Hook" label starts in the
// same column on every hook line: lifecycle lines spend their first two
// cells on the status glyph, inline tool indicator lines indent by the
// same two cells so both label columns line up.
func TestHookLabelColumnAlignment(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	lifecycle := ansi.Strip(NewHookItem(&sty, "msg-1", message.HookContent{
		Event: "PrePrompt",
		Hooks: []message.HookRun{{Event: "PrePrompt", Name: "slow-notify.sh", Decision: "none"}},
	}).RawRender(100))

	inline := &hooks.HookMetadata{
		Event:     hooks.EventPreToolUse,
		HookCount: 1,
		Hooks:     []hooks.HookInfo{{Name: "rtk-rewrite.sh", Matcher: "^bash$", Decision: "allow"}},
	}
	toolLine := ansi.Strip(toolOutputHookIndicator(&sty, hookMetaJSON(inline, nil), 100))

	// strings.Index is byte-based; strip everything from the label on to
	// measure the label column in display cells.
	labelCol := ansi.StringWidth(strings.SplitN(lifecycle, "Hook", 2)[0])
	require.Equal(t, 2, labelCol, "the lifecycle label sits two cells in, after the glyph")
	require.Equal(t, labelCol, ansi.StringWidth(strings.SplitN(toolLine, "Hook", 2)[0]),
		"the inline label must start in the same column as the lifecycle label")
	require.True(t, strings.HasPrefix(toolLine, "  Hook [PreTU]"),
		"the inline line indents by the glyph's two cells")
}

// TestHookLabelColumnAlignmentNarrowWidth pins the label-column contract
// on a width that dropped the status glyph: a lifecycle line whose
// glyphs cannot afford keeps the two-cell indent, so its Hook label
// still lines up with the inline indicator lines. The pre-fix code
// dropped the glyph and left the label at column 0.
func TestHookLabelColumnAlignmentNarrowWidth(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	width := 21
	lifecycle := ansi.Strip(NewHookItem(&sty, "msg-1", message.HookContent{
		Event: "PreSession",
		Hooks: []message.HookRun{{Event: "PreSession", Name: "session-start.sh", Decision: "none"}},
	}).RawRender(width))

	inline := &hooks.HookMetadata{
		Event:     hooks.EventPreToolUse,
		HookCount: 1,
		Hooks:     []hooks.HookInfo{{Name: "session-start.sh", Decision: "allow"}},
	}
	toolLine := ansi.Strip(toolOutputHookIndicator(&sty, hookMetaJSON(inline, nil), width))

	labelCol := ansi.StringWidth(strings.SplitN(lifecycle, "Hook", 2)[0])
	require.Equal(t, 2, labelCol, "the glyph-less lifecycle label keeps the two-cell indent")
	require.Equal(t, labelCol, ansi.StringWidth(strings.SplitN(toolLine, "Hook", 2)[0]),
		"the inline label must start in the same column")
}

// TestHookItemsAdjacentNoGap pins that consecutive hook items render as
// one block: the list-level gap row is suppressed between hook items,
// whatever their event types, while a non-hook neighbor keeps its gap.
func TestHookItemsAdjacentNoGap(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	first := NewHookItem(&sty, "msg-1", message.HookContent{
		Event: "PrePrompt",
		Hooks: []message.HookRun{{Event: "PrePrompt", Name: "first.sh", Decision: "none"}},
	})
	second := NewHookItem(&sty, "msg-2", message.HookContent{
		Event: "PostResponse",
		Hooks: []message.HookRun{{Event: "PostResponse", Name: "second.sh", Decision: "none"}},
	})

	l := list.NewList(first, second)
	l.SetSize(100, 20)
	l.SetGap(1)

	out := ansi.Strip(l.Render())
	lines := strings.Split(out, "\n")
	hookLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "Hook") {
			hookLines = append(hookLines, line)
		}
	}
	require.Len(t, hookLines, 2, "both hook lines render")
	firstIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "first.sh") {
			firstIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, firstIdx, 0)
	require.False(t, strings.Contains(lines[firstIdx+1], "Hook") && lines[firstIdx+1] == "",
		"no gap row may sit between two hook lines")
	require.True(t, strings.Contains(lines[firstIdx+1], "Hook"),
		"the second hook line follows immediately after the first")
}
