package cli

// Version is the build's version string. Overridden at link time via -ldflags.
// This is the single place to bump the version; see scripts/release.sh.
var Version = "0.3.8"

// Commit identifies the source revision, or unknown for an unstamped build.
var Commit = "unknown"

// BuildTime is the reproducible source timestamp supplied at link time.
var BuildTime = "unknown"
