package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The version const is the one piece of release metadata that is not derived
// from anything — it is typed by hand and reported to every MCP client, so
// nothing but a test notices when a release forgets to bump it. These tests
// pin it to the two things it is supposed to agree with.

var gobspectRequire = regexp.MustCompile(`(?m)^\s*github\.com/codepuke/gobspect\s+v(\S+)\s*$`)

// TestVersionMatchesGoMod enforces the policy that this server's version
// tracks the gobspect release it wraps. If a gobspect upgrade lands without a
// matching bump here, the number silently starts describing the wrong library.
func TestVersionMatchesGoMod(t *testing.T) {
	b, err := os.ReadFile("../../go.mod")
	require.NoError(t, err)

	m := gobspectRequire.FindSubmatch(b)
	require.NotNil(t, m, "no direct github.com/codepuke/gobspect requirement found in go.mod")

	require.Equal(t, string(m[1]), version,
		"version const and the gobspect version in go.mod disagree; see the release checklist in CLAUDE.md")
}

// TestVersionMatchesGitTag runs only on a tagged commit, which is exactly when
// getting this wrong ships. On every other commit there is no tag to agree
// with yet, so it skips rather than failing the ordinary test run.
func TestVersionMatchesGitTag(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	out, err := exec.Command("git", "describe", "--exact-match", "--tags", "HEAD").Output()
	if err != nil {
		t.Skip("HEAD is not tagged")
	}

	tag := strings.TrimSpace(string(out))
	require.Equal(t, "v"+version, tag,
		"tag %s does not match the version const %q; see the release checklist in CLAUDE.md", tag, version)
}

// TestServerVersion covers the reporting path itself. Under `go test` the main
// module has no stamped version, so this exercises the fallback.
func TestServerVersion(t *testing.T) {
	got := serverVersion()
	require.NotEmpty(t, got)
	require.False(t, strings.HasPrefix(got, "v"),
		"the reported version is unprefixed, so the build-info path must trim it")
}
