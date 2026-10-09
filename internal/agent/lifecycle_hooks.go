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

// fireRecordedLifecycleHook runs a lifecycle event's hooks asynchronously
// and records the run as a hook-only message in the session transcript,
// so the chat shows which hooks ran next to the turn. Decisions are
// ignored; a nil runner or an empty result records nothing.
func fireRecordedLifecycleHook(messages message.Service, runner *hooks.Runner, eventName, sessionID string, data hooks.SessionEventData) {
	if runner == nil || messages == nil {
		return
	}
	go func() {
		result, err := runner.RunLifecycle(context.Background(), eventName, sessionID, data)
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role: message.User,
			Parts: []message.ContentPart{
				message.HookContent{Event: eventName, Hooks: runs},
			},
		}); err != nil {
			slog.Error("Failed to record lifecycle hook run", "event", eventName, "session_id", sessionID, "error", err)
		}
	}()
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
