package fork

import (
	"errors"

	"github.com/charmbracelet/crush/internal/config"
)

// SetFlag persists the fork flag name in the global config under
// options.tui.fork.<name>, through upstream's generic SetConfigField.
func SetFlag(env Env, name string, enabled bool) error {
	return env.SetConfigField(config.ScopeGlobal, config.ForkFlagKey(name), enabled)
}

// ToggleFlag flips the fork flag name, whose unset value is def, persists
// the result and returns it.
func ToggleFlag(env Env, name string, def bool) (bool, error) {
	cfg := env.Config()
	if cfg == nil {
		return def, errors.New("configuration not found")
	}
	enabled := !cfg.ForkFlag(name, def)
	if err := SetFlag(env, name, enabled); err != nil {
		return !enabled, err
	}
	return enabled, nil
}
