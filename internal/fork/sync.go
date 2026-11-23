package fork

import (
	"sync"

	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
)

// SyncHook mirrors fork flags from the config into process state that
// cannot read the config on its own (package-level switches, fetch
// gates, render defaults).
type SyncHook func(cfg *config.Config)

var (
	syncHooksMu sync.RWMutex
	syncHooks   []SyncHook
)

// RegisterSyncHook adds a hook that Sync runs, in registration order. A
// nil hook is ignored.
func RegisterSyncHook(h SyncHook) {
	if h == nil {
		return
	}
	syncHooksMu.Lock()
	defer syncHooksMu.Unlock()
	syncHooks = append(syncHooks, h)
}

// Sync mirrors the fork flags into process state. The UI calls it at
// startup and after every fork command. It always drives the credits
// fetch gate (the balance only feeds the display, so a hidden display
// stops every fetch) and then runs the registered sync hooks.
func Sync(cfg *config.Config) {
	hyper.SetCreditsFetchEnabled(CreditsDisplayEnabled(cfg))
	syncHooksMu.RLock()
	hooks := append([]SyncHook(nil), syncHooks...)
	syncHooksMu.RUnlock()
	for _, h := range hooks {
		h(cfg)
	}
}
