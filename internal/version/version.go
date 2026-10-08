package version

import (
	"os"
	"regexp"
	"runtime/debug"
	"strconv"
)

// Build-time parameters set via -ldflags.

var (
	Version = "devel"
	Commit  = "unknown"
	// BuildNumber is set via -ldflags on fork builds. It counts the
	// builds of this fork checkout and lands in the version as the
	// -ly-<N> suffix.
	BuildNumber = ""
	// BuildID is a unique identifier for this build. For release builds it
	// equals Commit; for development builds (go run / go build without
	// ldflags) it is derived from the executable's modification time, which
	// changes on every recompilation.
	BuildID = ""
)

// baseVersionRegexp matches the upstream semver at the head of a build
// info version, which is either a plain tag or a pseudo version such as
// v0.97.2-0.20261008054726-c528f55323d2.
var baseVersionRegexp = regexp.MustCompile(`^v\d+\.\d+\.\d+`)

// forkVersion keeps the upstream semver and appends the fork marker
// -ly-<N>, where N is the fork build number. A dirty working tree and
// the commit hash no longer show in the version; both stay visible in
// the build info.
func forkVersion(mainVersion string) string {
	buildNumber := BuildNumber
	if buildNumber == "" {
		buildNumber = "0"
	}
	if base := baseVersionRegexp.FindString(mainVersion); base != "" {
		return base + "-ly-" + buildNumber
	}
	return mainVersion + "-ly"
}

// A user may install crush using `go install github.com/charmbracelet/crush@latest`.
// without -ldflags, in which case the version above is unset. As a workaround
// we use the embedded build version that *is* set when using `go install` (and
// is only set for `go install` and not for `go build`).
func init() {
	info, ok := debug.ReadBuildInfo()
	if ok {
		mainVersion := info.Main.Version
		if mainVersion != "" && mainVersion != "(devel)" {
			Version = forkVersion(mainVersion)
		}
	}

	// Derive BuildID when not set via ldflags.
	if BuildID == "" {
		BuildID = deriveBuildID()
	}
}

// deriveBuildID uses the running executable's modification time as a unique
// build fingerprint. This changes on every recompilation (including `go run`),
// making it reliable for detecting stale servers during development.
func deriveBuildID() string {
	exe, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	fi, err := os.Stat(exe)
	if err != nil {
		return "unknown"
	}
	return strconv.FormatInt(fi.ModTime().UnixNano(), 36)
}
