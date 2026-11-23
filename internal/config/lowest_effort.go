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
