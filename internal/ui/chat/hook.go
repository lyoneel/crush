package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// hookSeq disambiguates item ids across runs of the same event.
var hookSeq atomic.Int64

// lifecycleTag is the two-letter bracket tag shown in place of the full
// event name on lifecycle hook lines. The head must stay within the
// width budget of the body layout rendered by toolOutputHookIndicator.
var lifecycleTag = map[string]string{
	"SessionSwitch":   "[SS]",
	"SessionStart":    "[NS]",
	"PromptSubmitted": "[PS]",
	"AgentFinished":   "[AF]",
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

// NewHookItem builds a transcript item for a recorded lifecycle hook run.
func NewHookItem(sty *styles.Styles, content message.HookContent) MessageItem {
	v := list.NewVersioned()
	return &HookItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		id:                       fmt.Sprintf("hook-%d-%s", hookSeq.Add(1), content.Event),
		event:                    content.Event,
		hooks:                    content.Hooks,
		sty:                      sty,
	}
}

// ID implements MessageItem.
func (h *HookItem) ID() string { return h.id }

// FilterValue implements list filtering.
func (h *HookItem) FilterValue() string { return h.event + " hook" }

// Finished implements list.Item: the run record is static.
func (h *HookItem) Finished() bool { return true }

// HandleKeyEvent implements keyboard handling; hook items take no input.
func (h *HookItem) HandleKeyEvent(_ tea.KeyMsg) (bool, tea.Cmd) { return false, nil }

// Render implements list.Item.
func (h *HookItem) Render(width int) string {
	return h.RawRender(width)
}

// RawRender implements list.RawRenderable.
func (h *HookItem) RawRender(width int) string {
	infos := make([]hooks.HookInfo, len(h.hooks))
	for i, run := range h.hooks {
		infos[i] = hooks.HookInfo{Name: run.Name, Decision: run.Decision}
	}
	metadata, err := json.Marshal(struct {
		Hook *hooks.HookMetadata `json:"hook"`
	}{Hook: &hooks.HookMetadata{
		HookCount: len(infos),
		Hooks:     infos,
	}})
	if err != nil {
		return ansi.Truncate(h.event, max(width, 1), "…")
	}

	var head string
	if tag, ok := lifecycleTag[h.event]; ok {
		head = h.sty.Tool.HookLabel.Render("Hook ") + h.sty.Tool.HookTag.Render(tag)
	} else {
		head = h.sty.Tool.HookLabel.Render(strings.ToUpper(h.event[:1]) + h.event[1:] + " hooks")
	}
	body := toolOutputHookIndicator(h.sty, string(metadata), width)
	if body == "" {
		return head
	}
	return head + "\n" + body
}
