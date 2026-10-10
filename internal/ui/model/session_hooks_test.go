package model

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type hookWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w hookWorkspace) Config() *config.Config { return w.cfg }
func (w hookWorkspace) WorkingDir() string     { return tTempDirForHooks }

// tTempDirForHooks is set by the tests before the hook fires, because
// the async goroutine reads WorkingDir off the workspace.
var tTempDirForHooks string

func hookTestConfig(t *testing.T, command string) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	if command != "" {
		cfg.Hooks = map[string][]config.HookConfig{
			hooks.EventPreSessionSwitch: {{Command: command}},
		}
	}
	return cfg
}

// TestFireSessionSwitchHookRecordsBothSessions pins the switch payload:
// one call carries the abandoned session and the newly active one, and
// the synchronous command blocks until the hooks finish.
func TestFireSessionSwitchHookRecordsBothSessions(t *testing.T) {
	dir := t.TempDir()
	tTempDirForHooks = dir
	m := newTestUI()
	m.com.Workspace = hookWorkspace{cfg: hookTestConfig(t, "cat >> "+filepath.Join(dir, "hooks.log"))}

	old := &session.Session{ID: "sess-a", Title: "Old", PromptTokens: 7, Cost: 0.25}
	next := &session.Session{ID: "sess-b", Title: "New Session"}
	cmd := m.fireSessionSwitchHook(old, next, loadSessionMsg{})
	require.NotNil(t, cmd, "configured hooks must produce a command")
	msg := cmd()
	done, ok := msg.(switchHookDoneMsg)
	require.True(t, ok, "a synchronous switch hook must return a done message")

	logFile := filepath.Join(dir, "hooks.log")
	data, err := os.ReadFile(logFile)
	require.NoError(t, err, "a synchronous switch hook must finish before the command returns")
	require.Contains(t, string(data), `"event":"PreSessionSwitch"`)
	require.Contains(t, string(data), `"previous_session":{"id":"sess-a","title":"Old"`)
	require.Contains(t, string(data), `"new_session":{"id":"sess-b","title":"New Session"`)
	require.Len(t, done.runs, 1)
	require.Equal(t, "", done.runs[0].Status)
}

// TestFireSessionSwitchHookFirstSession pins the first-session shape:
// the abandoned side marshals as null.
func TestFireSessionSwitchHookFirstSession(t *testing.T) {
	dir := t.TempDir()
	tTempDirForHooks = dir
	m := newTestUI()
	m.com.Workspace = hookWorkspace{cfg: hookTestConfig(t, "cat >> "+filepath.Join(dir, "hooks.log"))}

	cmd := m.fireSessionSwitchHook(nil, &session.Session{ID: "sess-1"}, loadSessionMsg{})
	require.NotNil(t, cmd)
	cmd()

	logFile := filepath.Join(dir, "hooks.log")
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"previous_session":null`)
	require.Contains(t, string(data), `"new_session":{"id":"sess-1"`)
}

// TestFireSessionSwitchHookNoHooks pins the zero-configuration shape:
// nothing fires, nothing panics.
func TestFireSessionSwitchHookNoHooks(t *testing.T) {
	dir := t.TempDir()
	tTempDirForHooks = dir
	m := newTestUI()
	m.com.Workspace = hookWorkspace{cfg: hookTestConfig(t, "")}

	require.Nil(t, m.fireSessionSwitchHook(nil, &session.Session{ID: "sess-1"}, loadSessionMsg{}))
	require.Nil(t, m.fireSessionSwitchHook(&session.Session{ID: "a"}, &session.Session{ID: "b"}, loadSessionMsg{}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "no hook log may appear without configured hooks")
}

// TestSessionSwitched pins the switch detection: any first session is a
// switch, a same-id load is not.
func TestSessionSwitched(t *testing.T) {
	a := &session.Session{ID: "a"}
	b := &session.Session{ID: "b"}
	require.True(t, sessionSwitched(nil, b))
	require.True(t, sessionSwitched(a, b))
	require.False(t, sessionSwitched(a, &session.Session{ID: "a"}))
}

// TestFireSessionSwitchHookAsyncPendingThenDone pins the async shape:
// the batch carries a pending command (running status) and a completion
// command, and the completion carries the final statuses.
func TestFireSessionSwitchHookAsyncPendingThenDone(t *testing.T) {
	dir := t.TempDir()
	tTempDirForHooks = dir
	m := newTestUI()
	cfg := &config.Config{}
	cfg.Hooks = map[string][]config.HookConfig{
		hooks.EventPreSessionSwitch: {{Command: "cat >> " + filepath.Join(dir, "hooks.log"), Async: true}},
	}
	m.com.Workspace = hookWorkspace{cfg: cfg}

	cmd := m.fireSessionSwitchHook(&session.Session{ID: "a"}, &session.Session{ID: "b"}, loadSessionMsg{})
	require.NotNil(t, cmd)
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok, "an async switch fire returns a command batch")

	var pendingMsg switchHookPendingMsg
	var runsMsg switchHookRunsMsg
	for _, sub := range batch {
		switch msg := sub().(type) {
		case switchHookPendingMsg:
			pendingMsg = msg
		case switchHookRunsMsg:
			runsMsg = msg
		}
	}
	require.NotEmpty(t, pendingMsg.runs, "the pending command carries the running runs")
	require.Equal(t, hooks.StatusRunning, pendingMsg.runs[0].Status)
	require.True(t, pendingMsg.runs[0].Async, "the pending runs are marked async so the line carries the status glyph")
	require.Len(t, runsMsg.runs, 1)
	require.Equal(t, "", runsMsg.runs[0].Status, "the completion carries the final status")
	require.True(t, runsMsg.runs[0].Async, "the completion runs are marked async so the line carries the status glyph")

	logFile := filepath.Join(dir, "hooks.log")
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"PreSessionSwitch"`)
}

// TestUpdateSwitchHookLinesStaleGen pins the generation guard: a
// completion from an abandoned switch never updates the current lines.
func TestUpdateSwitchHookLinesStaleGen(t *testing.T) {
	tTempDirForHooks = t.TempDir()
	m := newTestUI()
	m.com.Workspace = hookWorkspace{cfg: hookTestConfig(t, "")}

	stale := switchHookRunsMsg{gen: switchHookGen.Load() - 1, runs: []message.HookRun{
		{Event: hooks.EventPreSessionSwitch, Name: "x.sh", Decision: "none"},
	}}
	m.updateSwitchHookLines(stale)
	require.Nil(t, m.chat.MessageItem(chat.HookItemID(switchHookLocalID, hooks.EventPreSessionSwitch)),
		"a stale completion must not create lines")
}
