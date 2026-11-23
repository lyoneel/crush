package config

import (
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

// TestLowestReasoningEffort pins the helper the fork uses to enforce the
// cheapest effort for small models: the first declared level, and no
// level at all for models that cannot reason.
func TestLowestReasoningEffort(t *testing.T) {
	t.Parallel()

	require.Equal(t, "low", LowestReasoningEffort(&catwalk.Model{
		Reasoning: catwalk.Reasoning{Thinking: catwalk.ThinkingToggleable, EffortLevels: []catwalk.EffortLevel{{Value: "low"}, {Value: "high"}, {Value: "xhigh"}}},
	}))
	require.Equal(t, "none", LowestReasoningEffort(&catwalk.Model{
		Reasoning: catwalk.Reasoning{Thinking: catwalk.ThinkingAlways, EffortLevels: []catwalk.EffortLevel{{Value: "none"}, {Value: "minimal"}, {Value: "low"}, {Value: "medium"}, {Value: "high"}}},
	}))
	require.Empty(t, LowestReasoningEffort(&catwalk.Model{}))
	require.Empty(t, LowestReasoningEffort(nil))
}

func TestLowestReasoningEffortCannotReason(t *testing.T) {
	t.Parallel()

	require.Empty(t, LowestReasoningEffort(&catwalk.Model{
		Reasoning: catwalk.Reasoning{Thinking: catwalk.ThinkingNever, EffortLevels: []catwalk.EffortLevel{{Value: "low"}}},
	}))
}

func TestSmallModelRunsAtLowestEffort(t *testing.T) {
	t.Parallel()

	cfg := &Config{Providers: csync.NewMapFrom(map[string]ProviderConfig{
		"p": {ID: "p", Models: []catwalk.Model{
			{ID: "reasoner", Reasoning: catwalk.Reasoning{Thinking: catwalk.ThinkingToggleable, EffortLevels: []catwalk.EffortLevel{{Value: "low"}, {Value: "high"}}}},
			{ID: "plain"},
		}},
	})}

	sel := SelectedModel{Provider: "p", Model: "reasoner", ReasoningEffort: "high"}
	require.Equal(t, "low", cfg.EffectiveSelection(SelectedModelTypeSmall, sel).ReasoningEffort)
	require.Equal(t, "high", cfg.EffectiveSelection(SelectedModelTypeLarge, sel).ReasoningEffort, "the large model keeps its effort")

	plain := SelectedModel{Provider: "p", Model: "plain", ReasoningEffort: "high"}
	require.Equal(t, "high", cfg.EffectiveSelection(SelectedModelTypeSmall, plain).ReasoningEffort)

	missing := SelectedModel{Provider: "p", Model: "missing", ReasoningEffort: "high"}
	require.Equal(t, "high", cfg.EffectiveSelection(SelectedModelTypeSmall, missing).ReasoningEffort)
}
