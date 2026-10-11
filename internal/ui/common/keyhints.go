package common

import (
	"runtime"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// symbolRule rewrites one canonical spelled-out prefix into the
// platform's symbol form. Rules apply in order, longest prefix first.
type symbolRule struct {
	from string
	to   string
}

// symbolRulesGeneric maps the canonical spellings to the symbol style
// on every non-darwin platform: Control uses the letters "ctl" (the
// Mac glyphs mean nothing there), Alt keeps its spelled-out form, and
// every modifier keeps the plus joining what follows.
var symbolRulesGeneric = []symbolRule{
	{"ctrl+shift+", "ctl+⇧+"}, // longest first: prefix order matters
	{"ctrl+", "ctl+"},
	{"alt+", "alt+"},
	{"shift+", "⇧+"},
}

// symbolRulesDarwin maps the canonical spellings to the symbol style on
// macOS: Alt renders as the Mac Option glyph while Control stays the
// letters "ctl" by fork decision. Every modifier keeps the plus.
var symbolRulesDarwin = []symbolRule{
	{"ctrl+shift+", "ctl+⇧+"}, // longest first: prefix order matters
	{"ctrl+", "ctl+"},
	{"alt+", "⌥+"},
	{"shift+", "⇧+"},
}

// activeSymbolRules is selected once at init from the platform and
// swapped by tests; the platform cannot change mid-process.
var activeSymbolRules = selectSymbolRules(runtime.GOOS)

// SymbolHintsEnabled reports whether the symbol style renders. The
// standalone TUIs (login, logout) hold no config, so they consult this
// package default; the main UI passes the config flag instead. A
// package-level variable keeps it test-swappable.
var SymbolHintsEnabled = true

func selectSymbolRules(goos string) []symbolRule {
	if goos == "darwin" {
		return symbolRulesDarwin
	}
	return symbolRulesGeneric
}

// arrowReplacements compacts spelled-out arrow pairs into single
// two-headed glyphs. The four-arrow run stays as the four individual
// arrows: the two-headed glyphs differ in size from the single-headed
// ones and render unclearly side by side.
var arrowReplacements = []symbolRule{
	{"↑/↓", "↕"},
	{"←/→", "↔"},
	{"↑↓", "↕"},
	{"←→", "↔"},
}

// TranslateHelp rewrites a binding's help key from the canonical
// spelled-out form to the platform's symbol form. The description
// passes through untouched, an empty key stays empty, and the function
// is idempotent: translating an already-symbolized hint changes
// nothing because the symbol forms carry no spelled-out prefixes.
func TranslateHelp(h key.Help) key.Help {
	if h.Key == "" {
		return h
	}
	out := h.Key
	for _, r := range activeSymbolRules {
		out = strings.ReplaceAll(out, r.from, r.to)
	}
	for _, r := range arrowReplacements {
		out = strings.ReplaceAll(out, r.from, r.to)
	}
	if out == "tab" {
		out = "⇆"
	}
	// A bare "enter" stays the word: 39 bindings declare it and the
	// spelled-out form is the fork's shipped look for it. Only the
	// enter word trailing a modifier glyph compacts ("⇧+enter" →
	// "⇧+⏎"); "⇧+enter" cannot appear otherwise because a bare enter
	// never carries a prefix.
	if h.Key != out && strings.HasSuffix(out, "enter") {
		out = strings.ReplaceAll(out, "enter", "⏎")
	}
	out = strings.ReplaceAll(out, "tab", "⇆")
	h.Key = out
	return h
}

// SymbolKeyMap wraps a help.KeyMap and translates every binding's help
// key to the platform's symbol form at render time. The wrapper
// returns translated copies and never mutates the inner map's
// bindings, so the same keymap renders correctly in both styles across
// toggles and re-renders.
type SymbolKeyMap struct {
	inner help.KeyMap
}

// NewSymbolKeyMap wraps the keymap for symbol rendering.
func NewSymbolKeyMap(inner help.KeyMap) SymbolKeyMap {
	return SymbolKeyMap{inner: inner}
}

// SymbolBindings translates a one-off binding slice: the inline help
// paths that build their binding list on the fly instead of holding a
// KeyMap.
func SymbolBindings(binds []key.Binding) []key.Binding {
	return translateBindings(binds)
}

// translateBindings returns translated copies of the bindings.
func translateBindings(binds []key.Binding) []key.Binding {
	out := make([]key.Binding, len(binds))
	for i, b := range binds {
		b.SetHelp(TranslateHelp(b.Help()).Key, b.Help().Desc)
		out[i] = b
	}
	return out
}

// ShortHelp implements help.KeyMap with translated help keys.
func (m SymbolKeyMap) ShortHelp() []key.Binding {
	return translateBindings(m.inner.ShortHelp())
}

// FullHelp implements help.KeyMap with translated help keys.
func (m SymbolKeyMap) FullHelp() [][]key.Binding {
	groups := m.inner.FullHelp()
	out := make([][]key.Binding, len(groups))
	for i, group := range groups {
		out[i] = translateBindings(group)
	}
	return out
}
