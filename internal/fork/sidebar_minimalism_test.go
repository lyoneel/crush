package fork

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsSidebarMinimalismDefaultsOn(t *testing.T) {
	t.Parallel()

	require.True(t, IsSidebarMinimalism(nil))
	require.True(t, IsSidebarMinimalism(&config.Config{}))

	off := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{
		Fork: map[string]bool{SidebarMinimalismFlag: false},
	}}}
	require.False(t, IsSidebarMinimalism(off))
}

func TestSidebarMinimalismCommand(t *testing.T) {
	t.Parallel()

	c, ok := LookupCommand("toggle_sidebar_minimalism")
	require.True(t, ok)
	require.Equal(t, "Toggle Compact Sidebar", c.Title(nil))

	env := &fakeEnv{cfg: &config.Config{}}
	info, err := c.Run(env)
	require.NoError(t, err)
	require.Equal(t, "Compact sidebar disabled", info)
	require.False(t, IsSidebarMinimalism(env.cfg))
	require.Equal(t, []string{"options.tui.fork.sidebar_minimalism"}, env.keys)

	info, err = c.Run(env)
	require.NoError(t, err)
	require.Equal(t, "Compact sidebar enabled", info)
	require.True(t, IsSidebarMinimalism(env.cfg))
}
