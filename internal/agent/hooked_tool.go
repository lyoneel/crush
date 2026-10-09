package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/tidwall/sjson"
)

// hookedTool wraps a fantasy.AgentTool to run PreToolUse hooks before
// delegating to the inner tool, and PostToolUse hooks after it returns.
type hookedTool struct {
	inner      fantasy.AgentTool
	runner     *hooks.Runner
	postRunner *hooks.Runner
}

func newHookedTool(inner fantasy.AgentTool, runner, postRunner *hooks.Runner) *hookedTool {
	return &hookedTool{inner: inner, runner: runner, postRunner: postRunner}
}

// wrapToolsWithHooks returns a tool slice with each entry wrapped in a
// hookedTool. Returns the original slice unchanged when both runners are
// nil or when isSubAgent is true — sub-agents never fire hooks, the
// top-level invocation of the sub-agent tool itself is wrapped on the
// caller's side.
func wrapToolsWithHooks(tools []fantasy.AgentTool, runner, postRunner *hooks.Runner, isSubAgent bool) []fantasy.AgentTool {
	if (runner == nil && postRunner == nil) || isSubAgent {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = newHookedTool(tool, runner, postRunner)
	}
	return out
}

func (h *hookedTool) Info() fantasy.ToolInfo {
	return h.inner.Info()
}

func (h *hookedTool) ProviderOptions() fantasy.ProviderOptions {
	return h.inner.ProviderOptions()
}

func (h *hookedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	h.inner.SetProviderOptions(opts)
}

func (h *hookedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := tools.GetSessionFromContext(ctx)

	var preResult hooks.AggregateResult
	if h.runner != nil {
		if hooks.AllAsync(h.runner.Hooks()) {
			// Fork: async PreToolUse fires fire-and-forget and ignores
			// every decision; the tool proceeds on the original input.
			input := call.Input
			go func() {
				if _, err := h.runner.Run(context.WithoutCancel(ctx), hooks.EventPreToolUse, sessionID, call.Name, input, ""); err != nil {
					slog.Warn("Hook execution error", "tool", call.Name, "error", err)
				}
			}()
		} else {
			result, err := h.runner.Run(ctx, hooks.EventPreToolUse, sessionID, call.Name, call.Input, "")
			if err != nil {
				slog.Warn("Hook execution error, proceeding with tool call",
					"tool", call.Name, "error", err)
			}
			preResult = result

			if result.Decision == hooks.DecisionDeny || result.Halt {
				reason := fmt.Sprintf("Tool call blocked by hook. Reason: %s", result.Reason)
				if result.Halt {
					reason = fmt.Sprintf("Turn halted by hook. Reason: %s", result.Reason)
				}
				resp := fantasy.NewTextErrorResponse(reason)
				// Halt ends the whole turn; a plain deny only blocks this tool
				// call so the model can see the error and try something else.
				resp.StopTurn = result.Halt
				resp.Metadata = hookMetadataJSON(result)
				return resp, nil
			}

			if result.UpdatedInput != "" {
				call.Input = result.UpdatedInput
			}

			// An explicit allow from a hook pre-approves the permission prompt for
			// this tool call. Deny is already handled above; silence falls through
			// to the normal permission flow.
			if result.Decision == hooks.DecisionAllow {
				ctx = permission.WithHookApproval(ctx, call.ID)
			}
		}
	}

	resp, err := h.inner.Run(ctx, call)

	output := ""
	if err != nil {
		output = err.Error()
	} else {
		output = resp.Content
	}

	if h.postRunner != nil && len(h.postRunner.Hooks()) > 0 {
		if hooks.AllAsync(h.postRunner.Hooks()) {
			// Fork: async PostToolUse fires fire-and-forget and renders
			// nothing; the already-returned response stays untouched.
			toolOutput := output
			go func() {
				if _, err := h.postRunner.Run(context.WithoutCancel(ctx), hooks.EventPostToolUse, sessionID, call.Name, call.Input, toolOutput); err != nil {
					slog.Warn("Hook execution error", "tool", call.Name, "error", err)
				}
			}()
		} else {
			postResult, postErr := h.postRunner.Run(ctx, hooks.EventPostToolUse, sessionID, call.Name, call.Input, output)
			if postErr != nil {
				slog.Warn("Hook execution error", "tool", call.Name, "error", postErr)
			}
			// Deny and input rewrite are ignored: the tool already ran.
			if postResult.Context != "" {
				if resp.Content != "" {
					resp.Content += "\n"
				}
				resp.Content += postResult.Context
			}
			if postResult.Halt {
				resp.StopTurn = true
			}
			resp.Metadata = mergePostHookMetadata(resp.Metadata, postResult)
		}
	}

	if err != nil {
		return resp, err
	}

	if preResult.Context != "" {
		if resp.Content != "" {
			resp.Content += "\n"
		}
		resp.Content += preResult.Context
	}

	resp.Metadata = mergeHookMetadata(resp.Metadata, preResult)
	return resp, nil
}

// buildHookMetadata creates a HookMetadata from an AggregateResult.
func buildHookMetadata(result hooks.AggregateResult) hooks.HookMetadata {
	return hooks.HookMetadata{
		Event:        hooks.EventPreToolUse,
		HookCount:    result.HookCount,
		Decision:     result.Decision.String(),
		Halt:         result.Halt,
		Reason:       result.Reason,
		InputRewrite: result.UpdatedInput != "",
		Hooks:        result.Hooks,
	}
}

// buildPostHookMetadata creates a PostToolUse HookMetadata from an
// AggregateResult.
func buildPostHookMetadata(result hooks.AggregateResult) hooks.HookMetadata {
	return hooks.HookMetadata{
		Event:     hooks.EventPostToolUse,
		HookCount: result.HookCount,
		Decision:  result.Decision.String(),
		Halt:      result.Halt,
		Reason:    result.Reason,
		Hooks:     result.Hooks,
	}
}

// hookMetadataJSON builds a JSON string containing only the hook metadata.
func hookMetadataJSON(result hooks.AggregateResult) string {
	meta := buildHookMetadata(result)
	data, err := json.Marshal(meta)
	if err != nil {
		return ""
	}
	return `{"hook":` + string(data) + `}`
}

// mergeHookMetadata injects hook metadata into existing tool metadata.
func mergeHookMetadata(existing string, result hooks.AggregateResult) string {
	if result.HookCount == 0 {
		return existing
	}
	meta := buildHookMetadata(result)
	data, err := json.Marshal(meta)
	if err != nil {
		return existing
	}
	if existing == "" {
		existing = "{}"
	}
	merged, err := sjson.SetRaw(existing, "hook", string(data))
	if err != nil {
		return existing
	}
	return merged
}

// mergePostHookMetadata injects PostToolUse hook metadata into existing
// tool metadata under a separate post_hook key, so the PreToolUse block
// and the PostToolUse block render as two sections in the same result.
func mergePostHookMetadata(existing string, result hooks.AggregateResult) string {
	if result.HookCount == 0 {
		return existing
	}
	meta := buildPostHookMetadata(result)
	data, err := json.Marshal(meta)
	if err != nil {
		return existing
	}
	if existing == "" {
		existing = "{}"
	}
	merged, err := sjson.SetRaw(existing, "post_hook", string(data))
	if err != nil {
		return existing
	}
	return merged
}
