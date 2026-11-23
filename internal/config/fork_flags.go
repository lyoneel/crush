package config

import "encoding/json"

// ForkFlagsKey is the config path that holds the fork's boolean feature
// flags. Each flag lives at options.tui.fork.<name>.
const ForkFlagsKey = "options.tui.fork"

// ForkFlagKey returns the full config path of the fork flag name, the key
// to hand to SetConfigField when persisting it.
func ForkFlagKey(name string) string {
	return ForkFlagsKey + "." + name
}

// legacyForkFlags maps the flat options.tui keys that earlier fork builds
// wrote to the flag names that replace them under options.tui.fork. A
// legacy value is honored only while the new flag is unset; saving a
// toggle always writes the new key.
var legacyForkFlags = map[string]string{
	"sidebar_minimalism": "sidebar_minimalism",
	"show_hypercredits":  "show_credits",
	"symbol_hints":       "symbol_hints",
}

// LegacyForkFlags returns a copy of the legacy key to flag name mapping,
// for documentation and tests.
func LegacyForkFlags() map[string]string {
	out := make(map[string]string, len(legacyForkFlags))
	for k, v := range legacyForkFlags {
		out[k] = v
	}
	return out
}

// ForkFlag reports the fork flag name, or def when it is unset. The nil
// receiver returns def, so callers can ask without unwrapping.
func (t *TUIOptions) ForkFlag(name string, def bool) bool {
	if t == nil {
		return def
	}
	if v, ok := t.Fork[name]; ok {
		return v
	}
	return def
}

// ForkFlag reports the fork flag name from the TUI options, or def when
// the config, its options or the flag are unset.
func (c *Config) ForkFlag(name string, def bool) bool {
	if c == nil || c.Options == nil {
		return def
	}
	return c.Options.TUI.ForkFlag(name, def)
}

// foldLegacyForkFlags copies boolean legacy keys found in the raw TUI
// options into the fork flag map, unless the new flag is already set.
// Folding at decode time keeps the value with the config wherever it
// travels, including the client/server config snapshot.
func foldLegacyForkFlags(t *TUIOptions, data []byte) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	for legacy, name := range legacyForkFlags {
		value, ok := raw[legacy]
		if !ok {
			continue
		}
		var enabled bool
		if err := json.Unmarshal(value, &enabled); err != nil {
			continue
		}
		if _, set := t.Fork[name]; set {
			continue
		}
		if t.Fork == nil {
			t.Fork = make(map[string]bool)
		}
		t.Fork[name] = enabled
	}
}
