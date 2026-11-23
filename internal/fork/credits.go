package fork

import (
	"sync"

	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
)

// CreditsDisplayGate reports whether the credits balance may show.
type CreditsDisplayGate func(cfg *config.Config) bool

var (
	creditsGatesMu sync.RWMutex
	creditsGates   []CreditsDisplayGate
)

// RegisterCreditsDisplayGate adds a gate on the credits display. The
// balance shows only while every registered gate allows it; with none
// registered it always may. A nil gate is ignored.
func RegisterCreditsDisplayGate(g CreditsDisplayGate) {
	if g == nil {
		return
	}
	creditsGatesMu.Lock()
	defer creditsGatesMu.Unlock()
	creditsGates = append(creditsGates, g)
}

// CreditsDisplayEnabled reports whether the credits balance may show, for
// every credits provider in the hyper registry.
func CreditsDisplayEnabled(cfg *config.Config) bool {
	creditsGatesMu.RLock()
	defer creditsGatesMu.RUnlock()
	for _, g := range creditsGates {
		if !g(cfg) {
			return false
		}
	}
	return true
}

// Sync mirrors the fork flags into process state that cannot read the
// config on its own. The UI calls it at startup and after every fork
// command. Today it drives the credits fetch gate: the balance only feeds
// the display, so a hidden display stops every fetch.
func Sync(cfg *config.Config) {
	hyper.SetCreditsFetchEnabled(CreditsDisplayEnabled(cfg))
}
