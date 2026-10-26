package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// TestFireLifecycleHookWritesPayload pins the end-to-end firing shape: a
// configured hook command receives the lifecycle payload on stdin.
func TestFireLifecycleHookWritesPayload(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "hooks.log")

	runner := hooks.NewRunner(
		[]config.HookConfig{{Command: "cat >> '" + logFile + "'"}},
		dir, dir,
	)
	result, err := runner.RunLifecycle(t.Context(), hooks.EventPrePrompt, "sess-1",
		hooks.SessionEventData{Prompt: "hello hook"})
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Hooks), "the single hook must report itself")

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
	logFile := filepath.Join(t.TempDir(), "hooks.log")
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPrePrompt: {{Command: "cat >> '" + logFile + "'"}},
	}

	fireLifecycleHookDefault(env.messages, cfg, hooks.EventPrePrompt, "sess-1",
		hooks.SessionEventData{Prompt: "hello hook"}, nil)

	data, err := os.ReadFile(logFile)
	require.NoError(t, err, "a sync hook must finish before the fire returns")
	require.Contains(t, string(data), `"event":"PrePrompt"`)
}

// TestFireLifecycleHookDefaultAsyncFiresAndForgets pins the async path:
// a hook with async: true returns immediately and its payload still
// lands in the log later.
func TestFireLifecycleHookDefaultAsyncFiresAndForgets(t *testing.T) {
	env := testEnv(t)
	logFile := filepath.Join(t.TempDir(), "hooks.log")
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPrePrompt: {{Command: "cat >> '" + logFile + "'", Async: true}},
	}

	fireLifecycleHookDefault(env.messages, cfg, hooks.EventPrePrompt, "sess-1",
		hooks.SessionEventData{Prompt: "hello hook"}, nil)

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(logFile)
		return err == nil && len(data) > 0
	}, 5*time.Second, 10*time.Millisecond, "an async hook must still run")
}

// TestFireLifecycleHookDefaultAsyncRecordsFinishedRun pins the per-hook
// async recording shape: an async: true hook fires fire-and-forget (the
// fire returns before the hook finishes), and the recorded run carries
// the async flag so the transcript renders the fire-and-forget glyph.
func TestFireLifecycleHookDefaultAsyncRecordsFinishedRun(t *testing.T) {
	env := testEnv(t)
	logFile := filepath.Join(t.TempDir(), "hooks.log")
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPrePrompt: {{Command: "cat >> '" + logFile + "'", Async: true}},
	}
	sess, err := env.sessions.Create(t.Context(), "hook session")
	require.NoError(t, err)

	fireLifecycleHookDefault(env.messages, cfg, hooks.EventPrePrompt, sess.ID,
		hooks.SessionEventData{Prompt: "hello hook"}, nil)

	// The fire returns immediately (the hook is fire-and-forget); the
	// transcript record may land in either order relative to the hook's
	// own completion. Eventually the record must exist, finished, with
	// the async flag set.
	require.Eventually(t, func() bool {
		msgs, err := env.messages.List(t.Context(), sess.ID)
		if err != nil {
			return false
		}
		for _, m := range msgs {
			if !m.OnlyHookParts() {
				continue
			}
			for _, part := range m.Parts {
				if hc, ok := part.(message.HookContent); ok {
					for _, h := range hc.Hooks {
						if h.Async && h.Status != hooks.StatusRunning {
							return true
						}
					}
				}
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "the async hook must record a finished run with the async flag")
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
	logFile := filepath.Join(t.TempDir(), "hooks.log")
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPreSession: {{Command: "cat >> '" + logFile + "'"}},
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
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	logFile := filepath.Join(t.TempDir(), "hooks.log")
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPostPrompt: {{Command: "cat >> '" + logFile + "'"}},
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

// TestUsageSourceFrom pins the snapshot conversion: session counters and
// model identity both ride; the empty form yields the zero source so
// the payload omits context_usage.
func TestUsageSourceFrom(t *testing.T) {
	src := usageSourceFrom(session.Session{
		ID:               "sess-1",
		PromptTokens:     12000,
		CompletionTokens: 800,
		EstimatedUsage:   true,
	}, Model{CatwalkCfg: catwalk.Model{ID: "glm-5.3-flash", ContextWindow: 200000}})
	require.Equal(t, int64(12000), src.SessionPromptTokens)
	require.Equal(t, int64(800), src.SessionCompletionTokens)
	require.True(t, src.SessionEstimatedUsage)
	require.Equal(t, "glm-5.3-flash", src.ModelID)
	require.Equal(t, int64(200000), src.ModelContextWindow)

	require.Equal(t, hooks.UsageSource{}, usageSourceFrom(session.Session{}, Model{}))
}
