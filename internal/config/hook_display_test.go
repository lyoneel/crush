package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHookConfigDisplayName pins the display-name fallback: an explicit
// name wins, a path command shows its base name, and an inline command
// shows in full.
func TestHookConfigDisplayName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "my-hook", (&HookConfig{
		Name:    "my-hook",
		Command: "/x/y.sh",
	}).DisplayName())

	// Regression: a path command without a name used to render as the
	// full command string in the TUI.
	require.Equal(t, "rtk-rewrite.sh", (&HookConfig{
		Command: "~/.config/crush/hooks/rtk-rewrite.sh",
	}).DisplayName())

	require.Equal(t, "a.sh", (&HookConfig{
		Command: "/home/u/hooks/a.sh",
	}).DisplayName())

	require.Equal(t, `echo '{"decision":"allow"}'`, (&HookConfig{
		Command: `echo '{"decision":"allow"}'`,
	}).DisplayName())
}
