package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// fakeTool records the context it was invoked with so tests can assert on
// values stamped onto it by the hookedTool decorator.
type fakeTool struct {
	name    string
	called  bool
	gotCtx  context.Context
	gotCall fantasy.ToolCall
	resp    fantasy.ToolResponse
	err     error
}

func (f *fakeTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: f.name}
}

func (f *fakeTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	f.called = true
	f.gotCtx = ctx
	f.gotCall = call
	return f.resp, f.err
}

func (f *fakeTool) ProviderOptions() fantasy.ProviderOptions     { return nil }
func (f *fakeTool) SetProviderOptions(_ fantasy.ProviderOptions) {}

// newRunner builds a hooks.Runner from a single HookConfig, running the
// config-loader path that compiles the matcher regex.
func newRunner(t *testing.T, cmd string) *hooks.Runner {
	t.Helper()
	return newRunnerForEvent(t, hooks.EventPreToolUse, config.HookConfig{Command: cmd})
}

// newRunnerForEvent builds a hooks.Runner for one event from a single
// HookConfig, running the config-loader path that compiles the matcher
// regex.
func newRunnerForEvent(t *testing.T, event string, hook config.HookConfig) *hooks.Runner {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			event: {hook},
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return hooks.NewRunner(cfg.Hooks[event], t.TempDir(), t.TempDir())
}

// stdinCapture builds a hook command that dumps the stdin payload to a
// file so tests can assert on the payload the hook received.
func stdinCapture(t *testing.T) (cmd, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "payload.json")
	return "cat > " + path, path
}

func TestHookedTool_AllowStampsHookApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `echo '{"decision":"allow"}'`)
	tool := newHookedTool(inner, runner, nil)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-1", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called, "inner tool should have run")

	// The inner tool's permission service can now treat call-1 as pre-approved.
	svc := permission.NewPermissionService(t.TempDir(), false, nil)
	granted, err := svc.Request(inner.gotCtx, permission.CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "call-1",
		ToolName:   "view",
		Action:     "read",
		Path:       t.TempDir(),
	})
	require.NoError(t, err)
	require.True(t, granted, "hook allow should bypass the permission prompt")
}

func TestHookedTool_SilentDoesNotStampApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `exit 0`) // no stdout, no decision
	tool := newHookedTool(inner, runner, nil)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-2", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called)

	// With no hook opinion, a fresh permission request has nothing stamped
	// and must fall through to the normal flow. We verify by checking that
	// the context does not look pre-approved for this call ID: sending a
	// request that no subscriber resolves will block until cancelled.
	svc := permission.NewPermissionService(t.TempDir(), false, nil)
	ctx, cancel := context.WithCancel(inner.gotCtx)
	cancel()
	granted, err := svc.Request(ctx, permission.CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "call-2",
		ToolName:   "view",
		Action:     "read",
		Path:       t.TempDir(),
	})
	require.Error(t, err, "no approval stamped => request should reach the prompt path")
	require.False(t, granted)
}

func TestHookedTool_DenySkipsInnerTool(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash"}
	runner := newRunner(t, `echo "blocked" >&2; exit 2`)
	tool := newHookedTool(inner, runner, nil)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-3", Name: "bash"})
	require.NoError(t, err)
	require.False(t, inner.called, "denied call must not reach the inner tool")
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "blocked")
}

func TestHookedTool_PostToolUseMergesMetadata(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("done")}
	postRunner := newRunnerForEvent(t, hooks.EventPostToolUse,
		config.HookConfig{Command: `echo '{"decision":"allow","context":"audited"}'`})
	tool := newHookedTool(inner, nil, postRunner)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-4", Name: "bash"})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.Contains(t, resp.Metadata, `"post_hook"`)
	require.Contains(t, resp.Metadata, hooks.EventPostToolUse)
	// Context injection appends the hook context to the response content.
	require.Contains(t, resp.Content, "done")
	require.Contains(t, resp.Content, "audited")
}

func TestHookedTool_PostToolUseAsyncSkipsMetadata(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("done")}
	postRunner := newRunnerForEvent(t, hooks.EventPostToolUse,
		config.HookConfig{Command: `echo '{"decision":"allow","context":"audited"}'`, Async: true})
	tool := newHookedTool(inner, nil, postRunner)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-5", Name: "bash"})
	require.NoError(t, err)
	require.NotContains(t, resp.Metadata, "post_hook")
	require.NotContains(t, resp.Content, "audited")
}

func TestHookedTool_PostToolUseMatcherFilters(t *testing.T) {
	t.Parallel()

	cmd, payloadPath := stdinCapture(t)
	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("done")}
	postRunner := newRunnerForEvent(t, hooks.EventPostToolUse,
		config.HookConfig{Command: cmd, Matcher: `^bash$`})
	tool := newHookedTool(inner, nil, postRunner)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-6", Name: "view"})
	require.NoError(t, err)
	require.NoFileExists(t, payloadPath, "hook must not run for a non-matching tool")

	_, err = tool.Run(t.Context(), fantasy.ToolCall{ID: "call-7", Name: "bash"})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(payloadPath)
		return statErr == nil
	}, 5*time.Second, 50*time.Millisecond, "hook must run for the matching tool")
}

func TestHookedTool_PostToolUseErrorPath(t *testing.T) {
	t.Parallel()

	cmd, payloadPath := stdinCapture(t)
	boom := errors.New("tool exploded")
	inner := &fakeTool{name: "bash", resp: fantasy.NewTextErrorResponse("tool exploded"), err: boom}
	postRunner := newRunnerForEvent(t, hooks.EventPostToolUse,
		config.HookConfig{Command: cmd})
	tool := newHookedTool(inner, nil, postRunner)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-8", Name: "bash", Input: `{"command":"ls"}`})
	require.Equal(t, boom, err, "the original tool error must return unchanged")
	require.True(t, resp.IsError)
	require.Eventually(t, func() bool {
		data, readErr := os.ReadFile(payloadPath)
		return readErr == nil && len(data) > 0
	}, 5*time.Second, 50*time.Millisecond, "hook must fire on the error path")
	data, readErr := os.ReadFile(payloadPath)
	require.NoError(t, readErr)
	require.Contains(t, string(data), `"tool_output":"tool exploded"`)
	require.Contains(t, string(data), `"tool_name":"bash"`)
}

func TestHookedTool_PostToolUseDenyAndRewriteIgnored(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("done")}
	postRunner := newRunnerForEvent(t, hooks.EventPostToolUse,
		config.HookConfig{Command: `echo '{"decision":"deny","reason":"too late","updatedInput":{"command":"rm -rf /"}}'`})
	tool := newHookedTool(inner, nil, postRunner)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-9", Name: "bash", Input: `{"command":"ls"}`})
	require.NoError(t, err)
	require.True(t, inner.called, "the tool already ran; a post deny cannot retract it")
	require.Equal(t, `{"command":"ls"}`, inner.gotCall.Input, "post rewrite must be ignored")
	require.False(t, resp.IsError)
	require.Contains(t, resp.Metadata, `"decision":"deny"`, "the decision still displays in the line")
}

func TestHookedTool_PostToolUseHaltStopsTurn(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("done")}
	postRunner := newRunnerForEvent(t, hooks.EventPostToolUse,
		config.HookConfig{Command: `echo '{"halt":true,"reason":"stop the turn"}'`})
	tool := newHookedTool(inner, nil, postRunner)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-10", Name: "bash"})
	require.NoError(t, err)
	require.True(t, resp.StopTurn, "a post-tool halt must end the turn after the tool ran")
}

func TestHookedTool_AsyncPreToolUseIgnoresDecisions(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("done")}
	runner := newRunnerForEvent(t, hooks.EventPreToolUse,
		config.HookConfig{Command: `echo '{"decision":"deny","reason":"nope"}'`, Async: true})
	tool := newHookedTool(inner, runner, nil)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-11", Name: "bash", Input: `{"command":"ls"}`})
	require.NoError(t, err, "async PreToolUse decisions are ignored")
	require.True(t, inner.called, "the tool proceeds on the original input")
	require.Equal(t, `{"command":"ls"}`, inner.gotCall.Input)
	require.False(t, resp.IsError)
}

func TestHookedTool_PostToolUsePayloadCarriesOutput(t *testing.T) {
	t.Parallel()

	cmd, payloadPath := stdinCapture(t)
	inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("tool said hi")}
	postRunner := newRunnerForEvent(t, hooks.EventPostToolUse,
		config.HookConfig{Command: cmd})
	tool := newHookedTool(inner, nil, postRunner)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-12", Name: "bash", Input: `{"command":"ls"}`})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		data, readErr := os.ReadFile(payloadPath)
		return readErr == nil && len(data) > 0
	}, 5*time.Second, 50*time.Millisecond)
	data, readErr := os.ReadFile(payloadPath)
	require.NoError(t, readErr)
	require.Contains(t, string(data), `"tool_output":"tool said hi"`)
	require.Contains(t, string(data), `"tool_input":{"command":"ls"}`)
}

func TestWrapToolsWithHooks(t *testing.T) {
	t.Parallel()

	runner := newRunner(t, `exit 0`)
	inputs := []fantasy.AgentTool{&fakeTool{name: "a"}, &fakeTool{name: "b"}}

	t.Run("top-level agent wraps every tool", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, nil, false)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})

	t.Run("sub-agent skips the wrap", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, nil, true)
		require.Equal(t, inputs, out, "sub-agent tools should be returned unwrapped")
		for _, tool := range out {
			_, isHooked := tool.(*hookedTool)
			require.False(t, isHooked, "sub-agent tool should not be wrapped")
		}
	})

	t.Run("nil runners skip the wrap for both agent kinds", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, false))
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, true))
	})

	t.Run("post-only hooks still wrap", func(t *testing.T) {
		t.Parallel()
		postRunner := newRunnerForEvent(t, hooks.EventPostToolUse, config.HookConfig{Command: `exit 0`})
		out := wrapToolsWithHooks(inputs, nil, postRunner, false)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})
}
