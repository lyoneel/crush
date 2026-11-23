package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEffectiveSelectionAppliesHooksInOrder(t *testing.T) {
	selectionHooksMu.Lock()
	saved := selectionHooks
	selectionHooks = nil
	selectionHooksMu.Unlock()
	t.Cleanup(func() {
		selectionHooksMu.Lock()
		selectionHooks = saved
		selectionHooksMu.Unlock()
	})

	sel := SelectedModel{Provider: "p", Model: "m", ReasoningEffort: "high"}
	require.Equal(t, sel, (&Config{}).EffectiveSelection(SelectedModelTypeSmall, sel))

	RegisterSelectionHook(nil)
	RegisterSelectionHook(func(_ *Config, slot SelectedModelType, s SelectedModel) SelectedModel {
		if slot == SelectedModelTypeSmall {
			s.ReasoningEffort = "low"
		}
		return s
	})
	RegisterSelectionHook(func(_ *Config, _ SelectedModelType, s SelectedModel) SelectedModel {
		s.ReasoningEffort += "!"
		return s
	})

	require.Equal(t, "low!", (&Config{}).EffectiveSelection(SelectedModelTypeSmall, sel).ReasoningEffort)
	require.Equal(t, "high!", (&Config{}).EffectiveSelection(SelectedModelTypeLarge, sel).ReasoningEffort)
	require.Equal(t, "high", sel.ReasoningEffort, "the input selection is not mutated")
}
