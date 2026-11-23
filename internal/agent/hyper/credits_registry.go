package hyper

import (
	"sync"
	"sync/atomic"
)

// CreditsMatcher reports whether a model, identified by its provider and
// model ids, consumes hypercredits.
type CreditsMatcher func(providerID, modelID string) bool

var (
	creditsMatchersMu sync.RWMutex
	creditsMatchers   = []CreditsMatcher{isHyperProvider}

	creditsFetchDisabled atomic.Bool
)

// isHyperProvider is the upstream rule: every model of the hyper provider
// itself consumes hypercredits.
func isHyperProvider(providerID, _ string) bool {
	return providerID == Name
}

// RegisterCreditsMatcher adds a matcher to the credits registry. Routers
// and gateways that serve Hyper models under another provider register
// one, usually from an init function, so the balance wrapper, the
// credits poll and the balance display apply to their models too. A nil
// matcher is ignored.
func RegisterCreditsMatcher(m CreditsMatcher) {
	if m == nil {
		return
	}
	creditsMatchersMu.Lock()
	defer creditsMatchersMu.Unlock()
	creditsMatchers = append(creditsMatchers, m)
}

// IsCreditsModel reports whether any registered matcher claims the model.
// The hyper provider always matches.
func IsCreditsModel(providerID, modelID string) bool {
	creditsMatchersMu.RLock()
	defer creditsMatchersMu.RUnlock()
	for _, m := range creditsMatchers {
		if m(providerID, modelID) {
			return true
		}
	}
	return false
}

// SetCreditsFetchEnabled turns the /v1/credits fetches on or off. The
// balance only feeds the display, so a hidden display skips every fetch:
// the UI poll and the per-request refreshes alike.
func SetCreditsFetchEnabled(enabled bool) {
	creditsFetchDisabled.Store(!enabled)
}

// CreditsFetchEnabled reports whether credits fetches run. On by default.
func CreditsFetchEnabled() bool {
	return !creditsFetchDisabled.Load()
}
