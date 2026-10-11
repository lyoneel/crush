package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type hookWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w hookWorkspace) Config() *config.Config { return w.cfg }
func (w hookWorkspace) WorkingDir() string     { return tTempDirForHooks }
func (w hookWorkspace) AgentModel() workspace.AgentModel {
	return workspace.AgentModel{}
}

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

// TestFireSessionSwitchHookAsyncHookRecordsFinishedRun pins the per-hook
// async shape: an async: true hook fires fire-and-forget inside the
// runner while the switch stays deferred on the synchronous completion;
// the recorded run carries the hook's own async flag.
func TestFireSessionSwitchHookAsyncHookRecordsFinishedRun(t *testing.T) {
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
	msg := cmd()
	done, ok := msg.(switchHookDoneMsg)
	require.True(t, ok, "the switch fire returns a single done command that carries the deferred load")
	require.Len(t, done.runs, 1, "the done message carries every run")
	require.True(t, done.runs[0].Async, "the run carries the hook's own async flag for the transcript glyph")
	require.Equal(t, hooks.EventPreSessionSwitch, done.runs[0].Event)

	logFile := filepath.Join(dir, "hooks.log")
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(logFile)
		return err == nil && strings.Contains(string(data), `"event":"PreSessionSwitch"`)
	}, 5*time.Second, 10*time.Millisecond, "the fire-and-forget hook must still receive the payload")
}
