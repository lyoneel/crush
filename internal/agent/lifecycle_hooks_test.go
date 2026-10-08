package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// TestFireLifecycleHookNilRunnerIsNoOp pins that a nil runner (no hooks
// configured) does nothing and does not panic.
func TestFireLifecycleHookNilRunnerIsNoOp(t *testing.T) {
	fireLifecycleHook(nil, hooks.EventPromptSubmitted, "sess-1", hooks.SessionEventData{Prompt: "hi"})
}

// TestFireLifecycleHookWritesPayload pins the end-to-end firing shape: a
// configured hook command receives the lifecycle payload on stdin.
func TestFireLifecycleHookWritesPayload(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "hooks.log")

	runner := hooks.NewRunner(
		[]config.HookConfig{{Command: "cat >> " + logFile}},
		dir, dir,
	)
	fireLifecycleHook(runner, hooks.EventPromptSubmitted, "sess-1",
		hooks.SessionEventData{Prompt: "hello hook"})

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(logFile)
		return err == nil && len(data) > 0
	}, 5*time.Second, 10*time.Millisecond)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"PromptSubmitted"`)
	require.Contains(t, string(data), `"prompt":"hello hook"`)
	require.Contains(t, string(data), `"session_id":"sess-1"`)
}

// TestSessionPayloadConversion pins the session snapshot mapping.
func TestSessionPayloadConversion(t *testing.T) {
	payload := sessionPayload(session.Session{
		ID:               "sess-1",
		Title:            "Title",
		MessageCount:     3,
		PromptTokens:     100,
		CompletionTokens: 50,
		Cost:             1.5,
		UpdatedAt:        42,
	})
	require.Equal(t, "sess-1", payload.ID)
	require.Equal(t, "Title", payload.Title)
	require.Equal(t, int64(3), payload.MessageCount)
	require.Equal(t, int64(100), payload.PromptTokens)
	require.Equal(t, int64(50), payload.CompletionTokens)
	require.Equal(t, 1.5, payload.Cost)
	require.Equal(t, int64(42), payload.UpdatedAt)
}

// TestLifecycleRunnerFromStore pins the nil-store behavior. The
// hooks-configured path is exercised through TestFireLifecycleHookWrites
// Payload, which builds the runner the same way NewRunner does.
func TestLifecycleRunnerFromStore(t *testing.T) {
	require.Nil(t, lifecycleRunnerFromStore(nil, hooks.EventAgentFinished))
}
