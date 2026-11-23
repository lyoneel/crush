package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fork"
	"github.com/stretchr/testify/require"
)

func TestCreditsDisplayToggleHidesBalance(t *testing.T) {
	t.Parallel()

	credits := 7
	cfg := &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: hyper.Name, Model: "m"},
		},
		Options: &config.Options{TUI: &config.TUIOptions{
			Fork: map[string]bool{fork.ShowCreditsFlag: false},
		}},
	}
	m := newTestUIWithConfig(t, cfg)
	m.hyperCredits = &credits
	require.True(t, m.com.IsCreditsModel())
	require.False(t, m.com.ShowCredits())
	require.Nil(t, m.visibleCredits())

	cfg.Options.TUI.Fork[fork.ShowCreditsFlag] = true
	require.Equal(t, &credits, m.visibleCredits())
}
