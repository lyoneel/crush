package fork

import "github.com/charmbracelet/crush/internal/config"

// SidebarMinimalismFlag is the fork flag behind the Toggle Compact Sidebar
// command: options.tui.fork.sidebar_minimalism, default on. The legacy
// flat key options.tui.sidebar_minimalism is honored while it is unset.
const SidebarMinimalismFlag = "sidebar_minimalism"

// IsSidebarMinimalism reports whether minimalist sidebar behaviors are
// on. Features that trim the sidebar ask it; the toggle does not need to
// know which of them are merged.
func IsSidebarMinimalism(cfg *config.Config) bool {
	return cfg.ForkFlag(SidebarMinimalismFlag, true)
}

func init() {
	RegisterCommand(Command{
		ID:    "toggle_sidebar_minimalism",
		Title: func(*config.Config) string { return "Toggle Compact Sidebar" },
		Run: func(env Env) (string, error) {
			enabled, err := ToggleFlag(env, SidebarMinimalismFlag, true)
			if err != nil {
				return "", err
			}
			if enabled {
				return "Compact sidebar enabled", nil
			}
			return "Compact sidebar disabled", nil
		},
	})
}
