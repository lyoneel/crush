package common

import (
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/stretchr/testify/require"
)

// keyHelp is a shorthand for building a key.Help value.
func keyHelp(k, desc string) key.Help {
	return key.Help{Key: k, Desc: desc}
}

// TestTranslateHelpGeneric pins the generic (non-darwin) table: every
// canonical form maps to its symbol form, Control as the letters "ctl"
// and Alt spelled out.
func TestTranslateHelpGeneric(t *testing.T) {
	prev := activeSymbolRules
	activeSymbolRules = selectSymbolRules("linux")
	t.Cleanup(func() { activeSymbolRules = prev })

	cases := []struct {
		in, want string
	}{
		{"ctrl+c", "ctl+c"},
		{"ctrl+g", "ctl+g"},
		{"ctrl+shift+v", "ctl+⇧+v"},
		{"shift+enter", "⇧+⏎"},
		{"shift+tab", "⇧+⇆"},
		{"shift+↑", "⇧+↑"},
		{"shift+←↓↑→", "⇧+←↓↑→"},
		{"alt+n", "alt+n"},
		{"alt+esc", "alt+esc"},
		{"↑/↓", "↕"},
		{"←/→", "↔"},
		{"↑↓", "↕"},
		{"shift+←↓↑→", "⇧+←↓↑→"},
		{"tab", "⇆"},
		{"esc", "esc"},
		{"space", "space"},
		{"f/pgdn", "f/pgdn"},
		{"enter", "enter"},
		{"y", "y"},
	}
	for _, c := range cases {
		got := TranslateHelp(keyHelp(c.in, "desc"))
		require.Equal(t, c.want, got.Key, "translating %q", c.in)
		require.Equal(t, "desc", got.Desc, "desc must pass through for %q", c.in)
	}
}

// TestTranslateHelpDarwin pins the darwin table: Control stays "ctl+",
// Alt renders as the Option glyph, both plus-joined.
func TestTranslateHelpDarwin(t *testing.T) {
	prev := activeSymbolRules
	activeSymbolRules = selectSymbolRules("darwin")
	t.Cleanup(func() { activeSymbolRules = prev })

	cases := []struct {
		in, want string
	}{
		{"ctrl+c", "ctl+c"},
		{"ctrl+shift+v", "ctl+⇧+v"},
		{"alt+n", "⌥+n"},
		{"shift+enter", "⇧+⏎"},
		{"shift+tab", "⇧+⇆"},
		{"↑/↓", "↕"},
		{"tab", "⇆"},
	}
	for _, c := range cases {
		got := TranslateHelp(keyHelp(c.in, "desc"))
		require.Equal(t, c.want, got.Key, "translating %q on darwin", c.in)
	}
}

// TestTranslateHelpIdempotent pins idempotence: translating an already
// translated hint changes nothing, on both tables.
func TestTranslateHelpIdempotent(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"linux", "darwin"} {
		prev := activeSymbolRules
		activeSymbolRules = selectSymbolRules(goos)
		once := TranslateHelp(keyHelp("ctrl+shift+enter", "d"))
		twice := TranslateHelp(once)
		require.Equal(t, once, twice, "translation must be idempotent on %s", goos)
		activeSymbolRules = prev
	}
}

// TestTranslateHelpEmptyKey pins the empty-key pass-through.
func TestTranslateHelpEmptyKey(t *testing.T) {
	t.Parallel()

	require.Equal(t, key.Help{}, TranslateHelp(key.Help{}))
}

// TestSelectSymbolRules pins the platform table selection.
func TestSelectSymbolRules(t *testing.T) {
	t.Parallel()

	require.Equal(t, symbolRulesDarwin, selectSymbolRules("darwin"))
	require.Equal(t, symbolRulesGeneric, selectSymbolRules("linux"))
	require.Equal(t, symbolRulesGeneric, selectSymbolRules("windows"))
}

// testKeyMap is a help.KeyMap stub returning fixed binding slices.
type testKeyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (m testKeyMap) ShortHelp() []key.Binding { return m.short }
func (m testKeyMap) FullHelp() [][]key.Binding {
	return m.full
}

// TestSymbolKeyMapReturnsTranslatedCopies pins the wrapper contract:
// the rendered bindings carry translated keys while the inner map's
// bindings keep the canonical spelling.
func TestSymbolKeyMapReturnsTranslatedCopies(t *testing.T) {
	t.Parallel()

	inner := testKeyMap{
		short: []key.Binding{
			key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		},
		full: [][]key.Binding{
			{
				key.NewBinding(key.WithKeys("shift+enter"), key.WithHelp("shift+enter", "newline")),
			},
		},
	}

	wrapped := NewSymbolKeyMap(inner)

	short := wrapped.ShortHelp()
	require.Equal(t, "ctl+c", short[0].Help().Key, "short help must translate")
	require.Equal(t, "quit", short[0].Help().Desc, "desc must pass through")
	require.Equal(t, "ctrl+c", inner.short[0].Help().Key, "the inner binding must stay canonical")

	full := wrapped.FullHelp()
	require.Equal(t, "⇧+⏎", full[0][0].Help().Key, "full help must translate")
	require.Equal(t, "shift+enter", inner.full[0][0].Help().Key, "the inner binding must stay canonical")
}
