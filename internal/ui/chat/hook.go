package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// lifecycleTag is the bracket tag shown in place of the full event name
// on hook lines. The tag rule is a Pre/Post/On prefix plus 2-3 uppercase
// letters. The legacy keys render transcript records that were written
// before the event rename; they must stay so old sessions keep their
// original tags.
var lifecycleTag = map[string]string{
	"PreToolUse":       "[PreTU]",
	"PostToolUse":      "[PostTU]",
	"PreSession":       "[PreSE]",
	"PreSessionSwitch": "[PreSS]",
	"PrePrompt":        "[PrePR]",
	"PostPrompt":       "[PostPR]",
	"PostResponse":     "[PostRE]",
	"SessionStart":     "[ST]",
	"SessionSwitch":    "[SS]",
	"PromptSubmitted":  "[PS]",
	"AgentFinished":    "[AF]",
}

// HookItem renders lifecycle hook runs recorded in the transcript: one
// line per hook, in the same shape as the PreToolUse tool indicator.
type HookItem struct {
	*list.Versioned
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	id    string
	event string
	hooks []message.HookRun
	sty   *styles.Styles
}

// HookItemID returns the chat item id for the hook content of one
// transcript message. The id is deterministic per message so a pending
// hook line can be updated in place when the run completes.
func HookItemID(msgID, event string) string {
	return fmt.Sprintf("hook-%s-%s", msgID, event)
}

// NewHookItem builds a transcript item for a recorded lifecycle hook run.
func NewHookItem(sty *styles.Styles, msgID string, content message.HookContent) MessageItem {
	v := list.NewVersioned()
	return &HookItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		id:                       HookItemID(msgID, content.Event),
		event:                    content.Event,
		hooks:                    content.Hooks,
		sty:                      sty,
	}
}

// SetContent replaces the recorded hook runs and re-renders. The list
// cache keys on the version, so the bump alone invalidates the entry.
func (h *HookItem) SetContent(content message.HookContent) {
	h.event = content.Event
	h.hooks = content.Hooks
	h.Bump()
	h.clearCache()
}

// ID implements MessageItem.
func (h *HookItem) ID() string { return h.id }

// FilterValue implements list filtering.
func (h *HookItem) FilterValue() string { return h.event + " hook" }

// Finished implements list.Item: the run record is static.
func (h *HookItem) Finished() bool { return true }

// NoGap implements list.NoGapItem: consecutive hook-run lines render as
// one block. The gap to any other item kind is unchanged.
func (h *HookItem) NoGap() bool { return true }

// HandleKeyEvent implements keyboard handling; hook items take no input.
func (h *HookItem) HandleKeyEvent(_ tea.KeyMsg) (bool, tea.Cmd) { return false, nil }

// Render implements list.Item.
func (h *HookItem) Render(width int) string {
	return h.RawRender(width)
}

// RawRender implements list.RawRenderable. The lifecycle run renders
// through the native tool hook indicator so each hook is one line in
// the shape `Hook [tag] name → OK`; the tag comes from the Event field
// set on the metadata, and an event without a tag renders the plain
// Hook label.
func (h *HookItem) RawRender(width int) string {
	infos := make([]hooks.HookInfo, len(h.hooks))
	for i, run := range h.hooks {
		infos[i] = hooks.HookInfo{Name: run.Name, Decision: run.Decision, Status: run.Status}
	}
	metadata, err := json.Marshal(struct {
		Hook *hooks.HookMetadata `json:"hook"`
	}{Hook: &hooks.HookMetadata{
		Event:     h.event,
		HookCount: len(infos),
		Hooks:     infos,
	}})
	if err != nil {
		return ansi.Truncate(h.event, max(width, 1), "…")
	}
	body := toolOutputHookIndicator(h.sty, string(metadata), width)
	if body == "" {
		return h.sty.Tool.HookLabel.Render(strings.ToUpper(h.event[:1]) + h.event[1:] + " hooks")
	}
	return body
}
