package fork

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsShowCreditsDefaultsOn(t *testing.T) {
	t.Parallel()

	require.True(t, IsShowCredits(nil))
	require.True(t, IsShowCredits(&config.Config{}))
}

func TestCreditsDisplayCommandGatesDisplayAndFetch(t *testing.T) {
	t.Cleanup(func() { hyper.SetCreditsFetchEnabled(true) })

	c, ok := LookupCommand("toggle_credits_display")
	require.True(t, ok)
	require.Equal(t, "Toggle Hypercredits", c.Title(nil))

	env := &fakeEnv{cfg: &config.Config{}}
	require.True(t, CreditsDisplayEnabled(env.cfg))

	info, err := c.Run(env)
	require.NoError(t, err)
	require.Equal(t, "Hypercredits hidden", info)
	require.Equal(t, []string{"options.tui.fork.show_credits"}, env.keys)
	require.False(t, CreditsDisplayEnabled(env.cfg))
	Sync(env.cfg)
	require.False(t, hyper.CreditsFetchEnabled())

	info, err = c.Run(env)
	require.NoError(t, err)
	require.Equal(t, "Hypercredits shown", info)
	require.True(t, CreditsDisplayEnabled(env.cfg))
	Sync(env.cfg)
	require.True(t, hyper.CreditsFetchEnabled())
}

func TestCreditsDisplayHonorsLegacyKey(t *testing.T) {
	t.Parallel()

	var tui config.TUIOptions
	require.NoError(t, tui.UnmarshalJSON([]byte(`{"show_hypercredits":false}`)))
	cfg := &config.Config{Options: &config.Options{TUI: &tui}}
	require.False(t, IsShowCredits(cfg))
	require.False(t, CreditsDisplayEnabled(cfg))
}
