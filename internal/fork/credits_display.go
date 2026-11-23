package fork

import "github.com/charmbracelet/crush/internal/config"

// ShowCreditsFlag is the fork flag behind the Toggle Hypercredits command:
// options.tui.fork.show_credits, default on. The legacy flat key
// options.tui.show_hypercredits is honored while it is unset.
const ShowCreditsFlag = "show_credits"

// IsShowCredits reports whether the credits balance display is on.
func IsShowCredits(cfg *config.Config) bool {
	return cfg.ForkFlag(ShowCreditsFlag, true)
}

func init() {
	// Gate the display, and through Sync the fetches, for every provider
	// in the credits registry.
	RegisterCreditsDisplayGate(IsShowCredits)

	RegisterCommand(Command{
		ID:    "toggle_credits_display",
		Title: func(*config.Config) string { return "Toggle Hypercredits" },
		Run: func(env Env) (string, error) {
			shown, err := ToggleFlag(env, ShowCreditsFlag, true)
			if err != nil {
				return "", err
			}
			if shown {
				return "Hypercredits shown", nil
			}
			return "Hypercredits hidden", nil
		},
	})
}
