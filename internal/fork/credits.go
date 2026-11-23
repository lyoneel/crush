package fork

import (
	"sync"

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
