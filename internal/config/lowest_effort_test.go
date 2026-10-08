package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

// TestLowestReasoningEffort pins the helper the fork uses to enforce the
// cheapest effort for small models: the first declared level, and no
// level at all for models that cannot reason.
func TestLowestReasoningEffort(t *testing.T) {
	t.Parallel()

	require.Equal(t, "low", LowestReasoningEffort(&catwalk.Model{
		CanReason:       true,
		ReasoningLevels: []string{"low", "high", "xhigh"},
	}))
	require.Equal(t, "none", LowestReasoningEffort(&catwalk.Model{
		CanReason:       true,
		ReasoningLevels: []string{"none", "minimal", "low", "medium", "high"},
	}))
	require.Empty(t, LowestReasoningEffort(&catwalk.Model{
		CanReason:       true,
		ReasoningLevels: nil,
	}))
	require.Empty(t, LowestReasoningEffort(&catwalk.Model{
		CanReason:       false,
		ReasoningLevels: []string{"low"},
	}))
	require.Empty(t, LowestReasoningEffort(nil))
}
