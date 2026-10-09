package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// lifecycleRunnerFromStore builds a hook runner for the given lifecycle
// event from the live config, or nil when the store is nil or no hooks
// are configured for the event.
func lifecycleRunnerFromStore(cfg *config.ConfigStore, eventName string) *hooks.Runner {
	if cfg == nil {
		return nil
	}
	eventHooks := cfg.Config().Hooks[eventName]
	if len(eventHooks) == 0 {
		return nil
	}
	return hooks.NewRunner(eventHooks, cfg.WorkingDir(), cfg.WorkingDir())
}

// fireLifecycleHook runs a lifecycle event's hooks asynchronously and
// ignores their decisions: lifecycle hooks are informational. A nil
// runner is a no-op; errors and hook context land in the log.
func fireLifecycleHook(runner *hooks.Runner, eventName, sessionID string, data hooks.SessionEventData) {
	hooks.FireAsync(runner, eventName, sessionID, data)
}

// runAndRecordLifecycleHook runs a lifecycle event's hooks and records the
// run as a hook-only message in the session transcript, so the chat shows
// which hooks ran next to the turn. Decisions are ignored; an empty result
// records nothing. The caller controls concurrency: pass a bounded context
// when the fire must complete before the caller continues.
func runAndRecordLifecycleHook(ctx context.Context, messages message.Service, runner *hooks.Runner, eventName, sessionID string, data hooks.SessionEventData) {
	if runner == nil || messages == nil {
		return
	}
	result, err := runner.RunLifecycle(ctx, eventName, sessionID, data)
	if err != nil {
		slog.Error("Lifecycle hook runner failed", "event", eventName, "session_id", sessionID, "error", err)
		return
	}
	if len(result.Hooks) == 0 {
		return
	}
	runs := make([]message.HookRun, len(result.Hooks))
	for i, h := range result.Hooks {
		runs[i] = message.HookRun{Event: eventName, Name: h.Name, Decision: h.Decision}
	}
	recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := messages.Create(recordCtx, sessionID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.HookContent{Event: eventName, Hooks: runs},
		},
	}); err != nil {
		slog.Error("Failed to record lifecycle hook run", "event", eventName, "session_id", sessionID, "error", err)
	}
}

// fireRecordedLifecycleHook runs a lifecycle event's hooks asynchronously
// and records the run as a hook-only message in the session transcript.
// Decisions are ignored; a nil runner or an empty result records nothing.
func fireRecordedLifecycleHook(messages message.Service, runner *hooks.Runner, eventName, sessionID string, data hooks.SessionEventData) {
	if runner == nil || messages == nil {
		return
	}
	go runAndRecordLifecycleHook(context.Background(), messages, runner, eventName, sessionID, data)
}

// FireNewSession fires the NewSession lifecycle hook for a freshly created
// session, asynchronously and with a transcript record. It is exported so
// both creation paths (server-mode backend and local app workspace) can
// share the fire; it is a no-op when no hooks are configured or the
// services are nil.
func FireNewSession(cfg *config.ConfigStore, sessions session.Service, messages message.Service, sess session.Session) {
	runner := lifecycleRunnerFromStore(cfg, hooks.EventNewSession)
	if runner == nil || messages == nil {
		return
	}
	data := hooks.SessionEventData{}
	if sessions != nil {
		if fresh, err := sessions.Get(context.Background(), sess.ID); err == nil {
			sess = fresh
		}
	}
	data.Session = sessionPayload(sess)
	fireRecordedLifecycleHook(messages, runner, hooks.EventNewSession, sess.ID, data)
}

// sessionPayload converts a session snapshot into the hook payload form.
func sessionPayload(s session.Session) *hooks.SessionPayload {
	return &hooks.SessionPayload{
		ID:               s.ID,
		Title:            s.Title,
		MessageCount:     s.MessageCount,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		Cost:             s.Cost,
		UpdatedAt:        s.UpdatedAt,
	}
}
