package agent

import (
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
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
