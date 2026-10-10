package hyper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsCreditsModel pins the credits-eligibility gate: the balance
// display follows the hyper provider itself and any gateway-served model
// whose id carries the Hyper/ prefix, and nothing else.
// TestIsCreditsModel pins the credits-eligibility gate: the balance
// display follows the hyper provider itself and any gateway-served model
// whose id carries the Hyper/ prefix in any letter case, and nothing else.
// TestIsCreditsModel pins the credits-eligibility gate: the balance
// display follows the hyper provider itself and any gateway-served model
// whose id carries the Hyper/ prefix in any letter case, or a /hyper/
// path segment in any letter case, and nothing else.
func TestIsCreditsModel(t *testing.T) {
	t.Parallel()

	require.True(t, IsCreditsModel("hyper", "glm-5.3-flash"))
	require.True(t, IsCreditsModel("hyper", ""))
	require.True(t, IsCreditsModel("GoModel-TITAN", "Hyper/glm-5.3-flash"))
	require.True(t, IsCreditsModel("some-gateway", "Hyper/deepseek-v4-pro"))
	require.True(t, IsCreditsModel("some-gateway", "Hyper/glm-5.3-flash"))
	require.True(t, IsCreditsModel("some-gateway", "hyPer/glm-5.3-flash"))
	require.True(t, IsCreditsModel("some-gateway", "HYPER/glm-5.3-flash"))
	require.True(t, IsCreditsModel("some-gateway", "hyper/glm-5.3-flash"))
	require.True(t, IsCreditsModel("some-gateway", "someservername/hyper/something"))
	require.True(t, IsCreditsModel("some-gateway", "Someservername/HYPER/Something"))
	require.True(t, IsCreditsModel("some-gateway", "a/hyper/b/hyper/c"))

	require.False(t, IsCreditsModel("GoModel-TITAN", "qwen/qwen3.8-27b"))
	require.False(t, IsCreditsModel("", ""))
	require.False(t, IsCreditsModel("hyper-clone", "glm-5.3-flash"))
	require.False(t, IsCreditsModel("GoModel-TITAN", "xhyper/glm-5.3-flash"))
	require.False(t, IsCreditsModel("GoModel-TITAN", "someservername/hyperextra/something"))
}
