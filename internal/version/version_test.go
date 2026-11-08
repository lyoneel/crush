package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestForkVersion pins the fork version scheme: the upstream semver from
// the build info stays untouched, and the fork marker -ly-<N> carries
// the build number, defaulting to 0 when no number was stamped.
func TestForkVersion(t *testing.T) {
	tests := []struct {
		name        string
		mainVersion string
		buildNumber string
		base        string
		want        string
	}{
		{
			name:        "pseudo version collapses to the upstream semver",
			mainVersion: "v0.97.2-0.20261008054726-c528f55323d2",
			buildNumber: "4",
			want:        "v0.97.2-ly-4",
		},
		{
			name:        "plain tag stays untouched apart from the marker",
			mainVersion: "v0.97.2",
			buildNumber: "4",
			want:        "v0.97.2-ly-4",
		},
		{
			name:        "stamped base wins over the build info version",
			mainVersion: "v0.97.2-0.20261008054726-c528f55323d2",
			buildNumber: "4",
			base:        "v0.97.1",
			want:        "v0.97.1-ly-4",
		},
		{
			name:        "missing build number defaults to 0",
			mainVersion: "v0.97.2-0.20261008054726-c528f55323d2",
			buildNumber: "",
			want:        "v0.97.2-ly-0",
		},
		{
			name:        "prerelease tag collapses to the base semver",
			mainVersion: "v1.2.3-rc1",
			buildNumber: "1",
			want:        "v1.2.3-ly-1",
		},
		{
			name:        "unrecognized version passes through with the marker",
			mainVersion: "weird",
			buildNumber: "1",
			want:        "weird-ly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalBase, originalNumber := Base, BuildNumber
			t.Cleanup(func() { Base, BuildNumber = originalBase, originalNumber })
			BuildNumber = tt.buildNumber
			Base = tt.base

			require.Equal(t, tt.want, forkVersion(tt.mainVersion))
		})
	}
}
