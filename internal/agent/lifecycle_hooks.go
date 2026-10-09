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
		runs[i] = message.HookRun{Event: eventName, Name: h.Name, Decision: h.Decision, Status: h.Status}
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
// It writes a pending record before the hooks start, so the chat shows a
// running line immediately, then updates that record in place when the
// run completes. Decisions are ignored; a nil runner or an empty result
// records nothing.
func fireRecordedLifecycleHook(messages message.Service, runner *hooks.Runner, eventName, sessionID string, data hooks.SessionEventData) {
	if runner == nil || messages == nil {
		return
	}
	pending := pendingLifecycleRuns(eventName, runner.Hooks())
	recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	msg, err := messages.Create(recordCtx, sessionID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.HookContent{Event: eventName, Hooks: pending},
		},
	})
	cancel()
	if err != nil {
		slog.Error("Failed to record pending lifecycle hook run", "event", eventName, "session_id", sessionID, "error", err)
	}
	go func() {
		runs := runLifecycleHookRuns(context.Background(), runner, eventName, sessionID, data)
		if len(runs) == 0 {
			if msg.ID != "" {
				delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer delCancel()
				if err := messages.Delete(delCtx, msg.ID); err != nil {
					slog.Error("Failed to delete pending lifecycle hook run", "event", eventName, "session_id", sessionID, "error", err)
				}
			}
			return
		}
		if msg.ID == "" {
			return
		}
		updated := msg.Clone()
		for i, part := range updated.Parts {
			if _, ok := part.(message.HookContent); ok {
				updated.Parts[i] = message.HookContent{Event: eventName, Hooks: runs}
			}
		}
		updCtx, updCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer updCancel()
		if err := messages.Update(updCtx, updated); err != nil {
			slog.Error("Failed to update lifecycle hook run", "event", eventName, "session_id", sessionID, "error", err)
		}
	}()
}

// pendingLifecycleRuns converts hook configs into pending transcript
// runs, one per hook, marked as still running.
func pendingLifecycleRuns(eventName string, hookCfgs []config.HookConfig) []message.HookRun {
	seen := make(map[string]bool, len(hookCfgs))
	runs := make([]message.HookRun, 0, len(hookCfgs))
	for _, h := range hookCfgs {
		if seen[h.Command] {
			continue
		}
		seen[h.Command] = true
		runs = append(runs, message.HookRun{Event: eventName, Name: h.DisplayName(), Decision: "none", Status: hooks.StatusRunning})
	}
	return runs
}

// runLifecycleHookRuns runs a lifecycle event's hooks and converts the
// result into transcript runs; an empty result records nothing.
func runLifecycleHookRuns(ctx context.Context, runner *hooks.Runner, eventName, sessionID string, data hooks.SessionEventData) []message.HookRun {
	result, err := runner.RunLifecycle(ctx, eventName, sessionID, data)
	if err != nil {
		slog.Error("Lifecycle hook runner failed", "event", eventName, "session_id", sessionID, "error", err)
		return nil
	}
	if len(result.Hooks) == 0 {
		return nil
	}
	runs := make([]message.HookRun, len(result.Hooks))
	for i, h := range result.Hooks {
		runs[i] = message.HookRun{Event: eventName, Name: h.Name, Decision: h.Decision, Status: h.Status}
	}
	return runs
}

// lifecycleHookTimeout caps a synchronous lifecycle fire. Each hook
// command already honors its own Timeout; this bounds the whole event.
const lifecycleHookTimeout = 30 * time.Second

// fireLifecycleHookDefault runs a lifecycle event's hooks, synchronous
// by default and recorded in the transcript. When every configured hook
// sets async: true the fire becomes fire-and-forget. Decisions are
// ignored; a nil runner or an empty result records nothing.
func fireLifecycleHookDefault(messages message.Service, cfg *config.ConfigStore, eventName, sessionID string, data hooks.SessionEventData) {
	runner := lifecycleRunnerFromStore(cfg, eventName)
	if runner == nil {
		return
	}
	if hooks.AllAsync(cfg.Config().Hooks[eventName]) {
		fireRecordedLifecycleHook(messages, runner, eventName, sessionID, data)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), lifecycleHookTimeout)
	defer cancel()
	runAndRecordLifecycleHook(ctx, messages, runner, eventName, sessionID, data)
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
