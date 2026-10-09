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
	fireLifecycleHook(nil, hooks.EventPrePrompt, "sess-1", hooks.SessionEventData{Prompt: "hi"})
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
	fireLifecycleHook(runner, hooks.EventPrePrompt, "sess-1",
		hooks.SessionEventData{Prompt: "hello hook"})

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(logFile)
		return err == nil && len(data) > 0
	}, 5*time.Second, 10*time.Millisecond)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"PrePrompt"`)
	require.Contains(t, string(data), `"prompt":"hello hook"`)
	require.Contains(t, string(data), `"session_id":"sess-1"`)
}

// TestFireLifecycleHookDefaultSyncWaits pins the sync default: the helper
// returns only after the hook command finished, so the payload file
// exists as soon as the call returns.
func TestFireLifecycleHookDefaultSyncWaits(t *testing.T) {
	env := testEnv(t)
	logFile := filepath.Join(env.workingDir, "hooks.log")
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPrePrompt: {{Command: "cat >> " + logFile}},
	}

	fireLifecycleHookDefault(env.messages, cfg, hooks.EventPrePrompt, "sess-1",
		hooks.SessionEventData{Prompt: "hello hook"})

	data, err := os.ReadFile(logFile)
	require.NoError(t, err, "a sync hook must finish before the fire returns")
	require.Contains(t, string(data), `"event":"PrePrompt"`)
}

// TestFireLifecycleHookDefaultAsyncFiresAndForgets pins the async path:
// a hook with async: true returns immediately and its payload still
// lands in the log later.
func TestFireLifecycleHookDefaultAsyncFiresAndForgets(t *testing.T) {
	env := testEnv(t)
	logFile := filepath.Join(env.workingDir, "hooks.log")
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPrePrompt: {{Command: "cat >> " + logFile, Async: true}},
	}

	fireLifecycleHookDefault(env.messages, cfg, hooks.EventPrePrompt, "sess-1",
		hooks.SessionEventData{Prompt: "hello hook"})

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(logFile)
		return err == nil && len(data) > 0
	}, 5*time.Second, 10*time.Millisecond, "an async hook must still run")
}

// TestFireLifecycleHookDefaultAsyncPendingThenDone pins the async
// recording shape: the fire writes a pending hook record immediately
// (so the chat can show a running line), then updates that same record
// in place with the final statuses when the run completes.
func TestFireLifecycleHookDefaultAsyncPendingThenDone(t *testing.T) {
	env := testEnv(t)
	logFile := filepath.Join(env.workingDir, "hooks.log")
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPrePrompt: {{Command: "cat >> " + logFile, Async: true}},
	}
	sess, err := env.sessions.Create(t.Context(), "hook session")
	require.NoError(t, err)

	fireLifecycleHookDefault(env.messages, cfg, hooks.EventPrePrompt, sess.ID,
		hooks.SessionEventData{Prompt: "hello hook"})

	var pendingID string
	require.Eventually(t, func() bool {
		msgs, err := env.messages.List(t.Context(), sess.ID)
		if err != nil || len(msgs) == 0 {
			return false
		}
		for _, m := range msgs {
			if !m.OnlyHookParts() {
				continue
			}
			for _, part := range m.Parts {
				if hc, ok := part.(message.HookContent); ok && len(hc.Hooks) > 0 {
					if hc.Hooks[0].Status != hooks.StatusRunning {
						return false
					}
					pendingID = m.ID
				}
			}
		}
		return pendingID != ""
	}, 5*time.Second, 10*time.Millisecond, "an async fire must record a pending hook line before the run completes")

	require.Eventually(t, func() bool {
		msg, err := env.messages.Get(t.Context(), pendingID)
		if err != nil {
			return false
		}
		for _, part := range msg.Parts {
			if hc, ok := part.(message.HookContent); ok {
				return len(hc.Hooks) == 1 && hc.Hooks[0].Status == ""
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "the pending record must update in place with the final status")
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
	require.Nil(t, lifecycleRunnerFromStore(nil, hooks.EventPostResponse))
}

// newPreSessionCoordinator wires a coordinator with a PreSession hook
// that records its payload, over a real temp-DB session and message service.
func newPreSessionCoordinator(t *testing.T) (*coordinator, string) {
	t.Helper()
	env := testEnv(t)
	logFile := filepath.Join(env.workingDir, "hooks.log")
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPreSession: {{Command: "cat >> " + logFile}},
	}
	return &coordinator{cfg: cfg, sessions: env.sessions, messages: env.messages}, logFile
}

// TestFirePreSessionFiresOnFirstPrompt pins the gate: a session with zero
// messages fires PreSession and records a HookContent message in the
// transcript. This test fails on the pre-fix code because the event does not
// exist.
func TestFirePreSessionFiresOnFirstPrompt(t *testing.T) {
	c, logFile := newPreSessionCoordinator(t)
	sess, err := c.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	c.firePreSession(t.Context(), sess.ID, "first prompt")

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"PreSession"`)
	require.Contains(t, string(data), `"prompt":"first prompt"`)

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
	require.Equal(t, hooks.EventPreSession, hook.Event)
	require.Len(t, hook.Hooks, 1)
}

// TestFirePreSessionSkipsAfterFirstPrompt pins the negative branch: once
// the session has a user text message, no further prompt fires the event.
func TestFirePreSessionSkipsAfterFirstPrompt(t *testing.T) {
	c, logFile := newPreSessionCoordinator(t)
	sess, err := c.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	_, err = c.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "first prompt"}},
	})
	require.NoError(t, err)

	c.firePreSession(t.Context(), sess.ID, "first prompt")

	_, err = os.Stat(logFile)
	require.True(t, os.IsNotExist(err), "no hook may run once the session has a user text message")
}

// TestFirePreSessionIgnoresHookOnlyMessages pins the gate refinement:
// hook-only transcript messages (such as a PrePrompt record) do not
// count as the session having been used, so PreSession still fires on
// the first prompt.
func TestFirePreSessionIgnoresHookOnlyMessages(t *testing.T) {
	c, logFile := newPreSessionCoordinator(t)
	sess, err := c.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	_, err = c.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.HookContent{Event: hooks.EventPrePrompt, Hooks: []message.HookRun{{Event: hooks.EventPrePrompt, Name: "h"}}}},
	})
	require.NoError(t, err)

	c.firePreSession(t.Context(), sess.ID, "first prompt")

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"PreSession"`)
}

// TestRunPostPromptRecorded pins the top-of-Run fire point: a prompt run
// through sessionAgent.Run records a PostPrompt HookContent message after
// the user prompt message is persisted. This test fails on the pre-change
// code because the PostPrompt event does not exist.
func TestRunPostPromptRecorded(t *testing.T) {
	sa, env := newStreamTestAgent(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	logFile := filepath.Join(env.workingDir, "hooks.log")
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPostPrompt: {{Command: "cat >> " + logFile}},
	}
	sa.cfg = cfg

	sess, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	_, runErr := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hello there"})
	require.NoError(t, runErr)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var postPrompt *message.HookContent
	var sawUserPrompt bool
	for _, m := range msgs {
		for _, part := range m.Parts {
			if hc, ok := part.(message.HookContent); ok && hc.Event == hooks.EventPostPrompt {
				postPrompt = &hc
			}
			if tc, ok := part.(message.TextContent); ok && tc.Text == "hello there" && m.Role == message.User {
				sawUserPrompt = true
			}
		}
	}
	require.True(t, sawUserPrompt, "the user prompt message must be persisted")
	require.NotNil(t, postPrompt, "the PostPrompt run must be recorded in the transcript")
	require.Len(t, postPrompt.Hooks, 1)
	require.Equal(t, hooks.EventPostPrompt, postPrompt.Hooks[0].Event)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"PostPrompt"`)
	require.Contains(t, string(data), `"prompt":"hello there"`)
}
