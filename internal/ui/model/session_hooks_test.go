package model

import (
	"os"
	"path/filepath"
	"testing"

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
// one call carries the abandoned session and the newly active one.
func TestFireSessionSwitchHookRecordsBothSessions(t *testing.T) {
	dir := t.TempDir()
	tTempDirForHooks = dir
	m := newTestUI()
	m.com.Workspace = hookWorkspace{cfg: hookTestConfig(t, "cat >> "+filepath.Join(dir, "hooks.log"))}

	old := &session.Session{ID: "sess-a", Title: "Old", PromptTokens: 7, Cost: 0.25}
	next := &session.Session{ID: "sess-b", Title: "New Session"}
	cmd := m.fireSessionSwitchHook(old, next)
	require.NotNil(t, cmd, "configured hooks must produce a command")
	msg := cmd()
	require.IsType(t, sessionHookStatusMsg{}, msg)

	logFile := filepath.Join(dir, "hooks.log")
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"event":"PreSessionSwitch"`)
	require.Contains(t, string(data), `"previous_session":{"id":"sess-a","title":"Old"`)
	require.Contains(t, string(data), `"new_session":{"id":"sess-b","title":"New Session"`)
}

// TestFireSessionSwitchHookFirstSession pins the first-session shape:
// the abandoned side marshals as null.
func TestFireSessionSwitchHookFirstSession(t *testing.T) {
	dir := t.TempDir()
	tTempDirForHooks = dir
	m := newTestUI()
	m.com.Workspace = hookWorkspace{cfg: hookTestConfig(t, "cat >> "+filepath.Join(dir, "hooks.log"))}

	cmd := m.fireSessionSwitchHook(nil, &session.Session{ID: "sess-1"})
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

	require.Nil(t, m.fireSessionSwitchHook(nil, &session.Session{ID: "sess-1"}))
	require.Nil(t, m.fireSessionSwitchHook(&session.Session{ID: "a"}, &session.Session{ID: "b"}))

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
