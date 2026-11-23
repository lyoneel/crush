package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGatewayHyperModelShowsCredits(t *testing.T) {
	t.Parallel()

	credits := 3
	cfg := &config.Config{Models: map[config.SelectedModelType]config.SelectedModel{
		config.SelectedModelTypeLarge: {Provider: "home-gateway", Model: "Hyper/glm-5.3-flash"},
	}}
	m := newTestUIWithConfig(t, cfg)
	m.hyperCredits = &credits
	require.True(t, m.com.ShowCredits())
	require.Equal(t, &credits, m.visibleCredits())
}
