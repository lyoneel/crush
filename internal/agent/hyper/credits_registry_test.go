package hyper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The registry is process-global, so these tests do not run in parallel
// and restore it when done.
func withCleanRegistry(t *testing.T) {
	t.Helper()
	creditsMatchersMu.Lock()
	saved := creditsMatchers
	creditsMatchers = []CreditsMatcher{isHyperProvider}
	creditsMatchersMu.Unlock()
	t.Cleanup(func() {
		creditsMatchersMu.Lock()
		creditsMatchers = saved
		creditsMatchersMu.Unlock()
	})
}

func TestIsCreditsModelDefault(t *testing.T) {
	withCleanRegistry(t)

	require.True(t, IsCreditsModel(Name, "any-model"))
	require.False(t, IsCreditsModel("openai", "gpt"))
	require.False(t, IsCreditsModel("gateway", "Hyper/model"))
}

func TestRegisterCreditsMatcher(t *testing.T) {
	withCleanRegistry(t)

	RegisterCreditsMatcher(nil)
	RegisterCreditsMatcher(func(providerID, modelID string) bool {
		return providerID == "gateway" && strings.HasPrefix(modelID, "Hyper/")
	})
	RegisterCreditsMatcher(func(providerID, _ string) bool {
		return providerID == "router-two"
	})

	require.True(t, IsCreditsModel(Name, "m"))
	require.True(t, IsCreditsModel("gateway", "Hyper/m"))
	require.True(t, IsCreditsModel("router-two", "x"))
	require.False(t, IsCreditsModel("gateway", "other/m"))
}

func TestCreditsFetchGate(t *testing.T) {
	t.Cleanup(func() { SetCreditsFetchEnabled(true) })

	require.True(t, CreditsFetchEnabled())
	SetCreditsFetchEnabled(false)
	require.False(t, CreditsFetchEnabled())

	credits, err := FetchCredits(t.Context(), "unused")
	require.NoError(t, err)
	require.Nil(t, credits, "a disabled gate must skip the request")

	SetCreditsFetchEnabled(true)
	require.True(t, CreditsFetchEnabled())
}
