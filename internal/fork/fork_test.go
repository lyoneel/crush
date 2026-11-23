package fork

import (
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// fakeEnv records SetConfigField calls and applies fork flags to its
// config, like the real store's reload does.
type fakeEnv struct {
	cfg  *config.Config
	keys []string
	err  error
}

func (f *fakeEnv) Config() *config.Config { return f.cfg }

func (f *fakeEnv) SetConfigField(scope config.Scope, key string, value any) error {
	if f.err != nil {
		return f.err
	}
	f.keys = append(f.keys, key)
	if f.cfg.Options == nil {
		f.cfg.Options = &config.Options{}
	}
	if f.cfg.Options.TUI == nil {
		f.cfg.Options.TUI = &config.TUIOptions{}
	}
	if f.cfg.Options.TUI.Fork == nil {
		f.cfg.Options.TUI.Fork = map[string]bool{}
	}
	name := key[len(config.ForkFlagsKey)+1:]
	f.cfg.Options.TUI.Fork[name] = value.(bool)
	return nil
}

func withCleanCommands(t *testing.T) {
	t.Helper()
	commandsMu.Lock()
	saved := commands
	commands = map[string]Command{}
	commandsMu.Unlock()
	t.Cleanup(func() {
		commandsMu.Lock()
		commands = saved
		commandsMu.Unlock()
	})
}

func noopRun(Env) (string, error) { return "", nil }

func TestRegisterCommandSortedAndLookup(t *testing.T) {
	withCleanCommands(t)

	title := func(*config.Config) string { return "t" }
	RegisterCommand(Command{ID: "zeta", Title: title, Run: noopRun})
	RegisterCommand(Command{ID: "alpha", Title: title, Run: noopRun})

	got := Commands()
	require.Len(t, got, 2)
	require.Equal(t, "alpha", got[0].ID)
	require.Equal(t, "zeta", got[1].ID)

	_, ok := LookupCommand("alpha")
	require.True(t, ok)
	_, ok = LookupCommand("missing")
	require.False(t, ok)
}

func TestRegisterCommandRejectsBadInput(t *testing.T) {
	withCleanCommands(t)

	title := func(*config.Config) string { return "t" }
	require.Panics(t, func() { RegisterCommand(Command{ID: " ", Title: title, Run: noopRun}) })
	require.Panics(t, func() { RegisterCommand(Command{ID: "x", Run: noopRun}) })
	require.Panics(t, func() { RegisterCommand(Command{ID: "x", Title: title}) })
	RegisterCommand(Command{ID: "x", Title: title, Run: noopRun})
	require.Panics(t, func() { RegisterCommand(Command{ID: "x", Title: title, Run: noopRun}) })
}

func TestToggleFlag(t *testing.T) {
	t.Parallel()

	env := &fakeEnv{cfg: &config.Config{}}
	enabled, err := ToggleFlag(env, "demo", true)
	require.NoError(t, err)
	require.False(t, enabled)
	require.Equal(t, []string{"options.tui.fork.demo"}, env.keys)

	enabled, err = ToggleFlag(env, "demo", true)
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestToggleFlagErrors(t *testing.T) {
	t.Parallel()

	_, err := ToggleFlag(&fakeEnv{}, "demo", true)
	require.Error(t, err)

	env := &fakeEnv{cfg: &config.Config{}, err: errors.New("boom")}
	enabled, err := ToggleFlag(env, "demo", true)
	require.Error(t, err)
	require.True(t, enabled, "a failed write reports the unchanged value")
}

func TestCreditsDisplayGateAndSync(t *testing.T) {
	creditsGatesMu.Lock()
	saved := creditsGates
	creditsGates = nil
	creditsGatesMu.Unlock()
	t.Cleanup(func() {
		creditsGatesMu.Lock()
		creditsGates = saved
		creditsGatesMu.Unlock()
		hyper.SetCreditsFetchEnabled(true)
	})

	require.True(t, CreditsDisplayEnabled(nil), "no gate means shown")

	RegisterCreditsDisplayGate(nil)
	RegisterCreditsDisplayGate(func(cfg *config.Config) bool { return cfg.ForkFlag("show", true) })

	cfg := &config.Config{}
	require.True(t, CreditsDisplayEnabled(cfg))
	Sync(cfg)
	require.True(t, hyper.CreditsFetchEnabled())

	cfg.Options = &config.Options{TUI: &config.TUIOptions{Fork: map[string]bool{"show": false}}}
	require.False(t, CreditsDisplayEnabled(cfg))
	Sync(cfg)
	require.False(t, hyper.CreditsFetchEnabled())
}

func TestSyncRunsHooksInOrder(t *testing.T) {
	syncHooksMu.Lock()
	saved := syncHooks
	syncHooks = nil
	syncHooksMu.Unlock()
	t.Cleanup(func() {
		syncHooksMu.Lock()
		syncHooks = saved
		syncHooksMu.Unlock()
	})

	var got []string
	RegisterSyncHook(nil)
	RegisterSyncHook(func(cfg *config.Config) { got = append(got, "a"+boolString(cfg.ForkFlag("x", false))) })
	RegisterSyncHook(func(*config.Config) { got = append(got, "b") })

	Sync(&config.Config{Options: &config.Options{TUI: &config.TUIOptions{Fork: map[string]bool{"x": true}}}})
	require.Equal(t, []string{"atrue", "b"}, got)
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
