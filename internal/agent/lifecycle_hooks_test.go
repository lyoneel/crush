package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
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

// newSessionStartCoordinator wires a coordinator with a SessionStart hook
// that records its payload, over a real temp-DB session and message service.
func newSessionStartCoordinator(t *testing.T) (*coordinator, string) {
	t.Helper()
	env := testEnv(t)
	logFile := filepath.Join(env.workingDir, "hooks.log")
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventSessionStart: {{Command: "cat >> " + logFile}},
	}
	return &coordinator{cfg: cfg, sessions: env.sessions, messages: env.messages}, logFile
}

// TestFireSessionStartFiresOnFirstPrompt pins the gate: a session with zero
// messages fires SessionStart and records a HookContent message in the
// transcript. This test fails on the pre-fix code because the event does not
// exist.
func TestFireSessionStartFiresOnFirstPrompt(t *testing.T) {
	c, logFile := newSessionStartCoordinator(t)
	sess, err := c.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	c.fireSessionStart(t.Context(), sess.ID)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"SessionStart"`)

	msgs, err := c.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	var hook *message.HookContent
	for _, part := range msgs[0].Parts {
		if hc, ok := part.(message.HookContent); ok {
			hook = &hc
		}
	}
	require.NotNil(t, hook, "the run must be recorded as a HookContent message")
	require.Equal(t, hooks.EventSessionStart, hook.Event)
	require.Len(t, hook.Hooks, 1)
}

// TestFireSessionStartSkipsAfterFirstPrompt pins the negative branch: once
// the session has a user text message, no further prompt fires the event.
func TestFireSessionStartSkipsAfterFirstPrompt(t *testing.T) {
	c, logFile := newSessionStartCoordinator(t)
	sess, err := c.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	_, err = c.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "first prompt"}},
	})
	require.NoError(t, err)

	c.fireSessionStart(t.Context(), sess.ID)

	_, err = os.Stat(logFile)
	require.True(t, os.IsNotExist(err), "no hook may run once the session has a user text message")
}

// TestFireSessionStartIgnoresHookOnlyMessages pins the gate refinement:
// hook-only transcript messages (such as the NewSession record written at
// creation) do not count as the session having been used, so SessionStart
// still fires on the first prompt.
func TestFireSessionStartIgnoresHookOnlyMessages(t *testing.T) {
	c, logFile := newSessionStartCoordinator(t)
	sess, err := c.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	_, err = c.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.HookContent{Event: hooks.EventNewSession, Hooks: []message.HookRun{{Event: hooks.EventNewSession, Name: "h"}}}},
	})
	require.NoError(t, err)

	c.fireSessionStart(t.Context(), sess.ID)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"SessionStart"`)
}

// TestFireNewSessionRecords pins the NewSession fire: a configured hook
// receives the payload and the run is recorded as a HookContent message in
// the fresh session's transcript.
func TestFireNewSessionRecords(t *testing.T) {
	env := testEnv(t)
	logFile := filepath.Join(env.workingDir, "hooks.log")
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventNewSession: {{Command: "cat >> " + logFile}},
	}

	sess, err := env.sessions.Create(t.Context(), "Fresh Session")
	require.NoError(t, err)

	FireNewSession(cfg, env.sessions, env.messages, sess)

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(logFile)
		return err == nil && len(data) > 0
	}, 5*time.Second, 10*time.Millisecond)
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"NewSession"`)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var hook *message.HookContent
	for _, msg := range msgs {
		for _, part := range msg.Parts {
			if hc, ok := part.(message.HookContent); ok {
				hook = &hc
			}
		}
	}
	require.NotNil(t, hook, "the NewSession run must be recorded as a HookContent message")
	require.Equal(t, hooks.EventNewSession, hook.Event)
}

// TestFireNewSessionNoHooksIsNoOp pins that with no configured hooks the
// fire does nothing and writes no transcript message.
func TestFireNewSessionNoHooksIsNoOp(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	sess, err := env.sessions.Create(t.Context(), "Fresh Session")
	require.NoError(t, err)

	FireNewSession(cfg, env.sessions, env.messages, sess)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, msgs)
}
