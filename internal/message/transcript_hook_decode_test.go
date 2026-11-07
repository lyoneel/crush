package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUnknownHookPartTypeDecoding documents the purge-time behavior: a
// persisted message whose parts carry the removed "hook" type tag must
// fail loudly (unknown part type) rather than crash or silently drop
// siblings. Q002 of the purge plan.
func TestUnknownHookPartTypeDecoding(t *testing.T) {
	data := `[{"type":"hook","data":{"event":"PrePrompt","hooks":[]}},{"type":"text","data":{"text":"hello"}}]`
	_, err := unmarshalParts([]byte(data))
	require.Error(t, err)
	require.ErrorContains(t, err, "unknown part type")
}
