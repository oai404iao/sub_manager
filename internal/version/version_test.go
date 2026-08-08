package version

import "testing"

func TestCurrentUsesFallbacks(t *testing.T) {
	previousVersion, previousCommit, previousBuildDate := Version, Commit, BuildDate
	t.Cleanup(func() {
		Version, Commit, BuildDate = previousVersion, previousCommit, previousBuildDate
	})

	Version, Commit, BuildDate = "", "", ""
	got := Current()
	if got.Version != "dev" || got.Commit != "unknown" || got.BuildDate != "unknown" {
		t.Fatalf("Current() = %#v", got)
	}
}

func TestString(t *testing.T) {
	previousVersion, previousCommit, previousBuildDate := Version, Commit, BuildDate
	t.Cleanup(func() {
		Version, Commit, BuildDate = previousVersion, previousCommit, previousBuildDate
	})

	Version, Commit, BuildDate = "0.1.0", "abc123", "2026-08-08T00:00:00Z"
	const want = "sub-manager 0.1.0 (commit abc123, built 2026-08-08T00:00:00Z)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
