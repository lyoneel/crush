package config

import "charm.land/catwalk/v3/pkg/catwalk"

// LowestReasoningEffort returns the lowest configured reasoning level
// for the model, or the empty string when the model cannot reason or
// declares no levels. The catalogs order the levels from lowest to
// highest, so the first one is the cheapest.
func LowestReasoningEffort(model *catwalk.Model) string {
	if model == nil || !model.CanReason() || len(model.Reasoning.EffortLevels) == 0 {
		return ""
	}
	return model.Reasoning.EffortLevels[0].Value
}

// smallModelLowestEffort is the selection hook that runs the small model
// at its lowest declared reasoning level, so background calls (titles,
// summaries, sub-agent chores) stay cheap regardless of the selected
// effort. A model that cannot reason, or one missing from the catalog,
// passes through.
func smallModelLowestEffort(cfg *Config, slot SelectedModelType, sel SelectedModel) SelectedModel {
	if slot != SelectedModelTypeSmall || cfg == nil {
		return sel
	}
	if effort := LowestReasoningEffort(cfg.GetModel(sel.Provider, sel.Model)); effort != "" {
		sel.ReasoningEffort = effort
	}
	return sel
}

func init() {
	RegisterSelectionHook(smallModelLowestEffort)
}
