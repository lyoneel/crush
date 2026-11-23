package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fork"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestForkCommandItemsListsRegisteredCommands(t *testing.T) {
	t.Parallel()

	fork.RegisterCommand(fork.Command{
		ID: "dialog_test_command",
		Title: func(cfg *config.Config) string {
			if cfg.ForkFlag("dialog_test", true) {
				return "Disable Test"
			}
			return "Enable Test"
		},
		Run: func(fork.Env) (string, error) { return "", nil },
	})

	s := styles.ThemeFromConfig("")
	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{Fork: map[string]bool{"dialog_test": false}}}}
	var found *CommandItem
	for _, item := range forkCommandItems(&s, cfg) {
		if item.ID() == "fork_dialog_test_command" {
			found = item
		}
	}
	require.NotNil(t, found)
	require.Equal(t, "Enable Test", found.title)
	require.Equal(t, ActionRunForkCommand{ID: "dialog_test_command"}, found.Action())
}
