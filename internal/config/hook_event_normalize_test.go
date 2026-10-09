package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeHookEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"pretooluse", "PreToolUse"},
		{"post_tool_use", "PostToolUse"},
		{"pre_session", "PreSession"},
		{"pre_session_switch", "PreSessionSwitch"},
		{"pre_prompt", "PrePrompt"},
		{"post_prompt", "PostPrompt"},
		{"post_response", "PostResponse"},
		{"SessionStart", "SessionStart"},
		{"unknown_event", "unknown_event"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, normalizeHookEvent(tc.in), tc.in)
	}
}
