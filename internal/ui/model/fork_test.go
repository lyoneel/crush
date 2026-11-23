package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestVisibleCreditsFollowsCreditsModel(t *testing.T) {
	t.Parallel()

	credits := 42
	hyperCfg := &config.Config{Models: map[config.SelectedModelType]config.SelectedModel{
		config.SelectedModelTypeLarge: {Provider: hyper.Name, Model: "m"},
	}}
	m := newTestUIWithConfig(t, hyperCfg)
	m.hyperCredits = &credits
	require.True(t, m.com.ShowCredits())
	require.Equal(t, &credits, m.visibleCredits())

	otherCfg := &config.Config{Models: map[config.SelectedModelType]config.SelectedModel{
		config.SelectedModelTypeLarge: {Provider: "openai", Model: "gpt"},
	}}
	m = newTestUIWithConfig(t, otherCfg)
	m.hyperCredits = &credits
	require.False(t, m.com.ShowCredits())
	require.Nil(t, m.visibleCredits(), "a model outside the credits registry shows no balance")
}
