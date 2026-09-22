package release_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/changelog"
	"github.com/cybersafetyid/featherctl/internal/release"
)

const changelogWithUnreleased = `# Changelog

## [Unreleased]

### Added

- a brand new thing

[Unreleased]: https://github.com/cybersafetyid/featherctl/commits/main
`

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	for _, valid := range []string{"0.1.0", "v0.1.0", "1.0.0-rc.1", "1.2.3+build.4", " 2.0.0 "} {
		_, err := release.NormalizeVersion(valid)
		assert.NoError(t, err, valid)
	}

	for _, invalid := range []string{"", "1.0", "v", "latest", "1.0.0.0", "1.x.0"} {
		_, err := release.NormalizeVersion(invalid)
		assert.Error(t, err, invalid)
	}

	version, err := release.NormalizeVersion("v1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", version)
}

func TestBump(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)

	var out bytes.Buffer
	result, err := bump(t, release.Options{
		Dir:     repo,
		Version: "v0.1.0",
		Date:    "2026-09-22",
		RepoURL: "https://github.com/cybersafetyid/featherctl",
		Out:     &out,
	})
	require.NoError(t, err)

	assert.True(t, result.Promoted)
	assert.True(t, result.Committed)
	assert.True(t, result.Tagged)
	assert.False(t, result.Pushed)
	assert.Empty(t, result.Previous)
	assert.Equal(t, "### Added\n\n- a brand new thing", result.Notes)

	assert.Equal(t, `# Changelog

## [Unreleased]

## [0.1.0] - 2026-09-22

### Added

- a brand new thing

[Unreleased]: https://github.com/cybersafetyid/featherctl/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/cybersafetyid/featherctl/releases/tag/v0.1.0
`, readFile(t, filepath.Join(repo, release.ChangelogFile)))

	assert.Equal(t, "chore(release): v0.1.0", gitOut(t, repo, "log", "-1", "--pretty=%s"))
	assert.Equal(t, "v0.1.0", gitOut(t, repo, "tag", "--list", "v0.1.0"))
	assert.Equal(t, "tag", gitOut(t, repo, "cat-file", "-t", "v0.1.0"), "the tag must be annotated")
	assert.Equal(t, "", gitOut(t, repo, "status", "--porcelain"))
	assert.Contains(t, out.String(), "prepared  section [0.1.0] - 2026-09-22")
}

func TestBumpSecondReleaseComparesAgainstThePreviousTag(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)

	_, err := bump(t, release.Options{
		Dir: repo, Version: "0.1.0", Date: "2026-09-22",
		RepoURL: "https://github.com/cybersafetyid/featherctl",
	})
	require.NoError(t, err)

	// The next cycle: document a fix under [Unreleased] and release again.
	writeFile(t, filepath.Join(repo, release.ChangelogFile), strings.Replace(
		readFile(t, filepath.Join(repo, release.ChangelogFile)),
		"## [Unreleased]\n\n## [0.1.0]",
		"## [Unreleased]\n\n### Fixed\n\n- a fix\n\n## [0.1.0]",
		1,
	))
	commitAll(t, repo, "docs: note the fix")

	result, err := bump(t, release.Options{
		Dir: repo, Version: "0.2.0", Date: "2026-10-01",
	})
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", result.Previous)
	assert.Equal(t, "### Fixed\n\n- a fix", result.Notes)

	content := readFile(t, filepath.Join(repo, release.ChangelogFile))
	assert.Contains(t, content, "[0.2.0]: https://github.com/cybersafetyid/featherctl/compare/v0.1.0...v0.2.0")
	assert.Contains(t, content, "[Unreleased]: https://github.com/cybersafetyid/featherctl/compare/v0.2.0...HEAD")
	assert.Contains(t, content, "## [0.1.0] - 2026-09-22")
}

func TestBumpKeepsAnExistingSection(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)

	// A maintainer who prefers to write the section by hand can do so; bump then
	// only commits and tags.
	documented := strings.Replace(
		readFile(t, filepath.Join(repo, release.ChangelogFile)),
		"## [Unreleased]", "## [Unreleased]\n\n## [0.3.0] - 2026-08-01", 1,
	)
	writeFile(t, filepath.Join(repo, release.ChangelogFile), documented)
	commitAll(t, repo, "docs: document 0.3.0 by hand")

	result, err := bump(t, release.Options{Dir: repo, Version: "0.3.0"})
	require.NoError(t, err)

	assert.False(t, result.Promoted, "the hand-written section is kept as it is")
	assert.True(t, result.Tagged)
	assert.Equal(t, "### Added\n\n- a brand new thing", result.Notes)

	// The section is untouched; only the trailing links move to the new tag.
	content := readFile(t, filepath.Join(repo, release.ChangelogFile))
	assert.Contains(t, content, "## [0.3.0] - 2026-08-01\n\n### Added\n\n- a brand new thing")
	assert.Contains(t, content, "[0.3.0]: https://github.com/cybersafetyid/featherctl/releases/tag/v0.3.0")
	assert.Equal(t, "", gitOut(t, repo, "status", "--porcelain"))
}

func TestBumpFallsBackToTheModulePathForLinks(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)
	gitOut(t, repo, "config", "--unset", "remote.origin.url")
	writeFile(t, filepath.Join(repo, "go.mod"), "module github.com/cybersafetyid/featherctl\n\ngo 1.22.0\n")

	result, err := bump(t, release.Options{Dir: repo, Version: "0.1.0", Date: "2026-09-22"})
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/cybersafetyid/featherctl", result.RepoURL)

	content := readFile(t, filepath.Join(repo, release.ChangelogFile))
	assert.Contains(t, content, "[0.1.0]: https://github.com/cybersafetyid/featherctl/releases/tag/v0.1.0")
}

func TestBumpWithoutAnyRepositoryURLLeavesTheLinksAlone(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)
	gitOut(t, repo, "config", "--unset", "remote.origin.url")

	var out bytes.Buffer
	result, err := bump(t, release.Options{Dir: repo, Version: "0.1.0", Date: "2026-09-22", Out: &out})
	require.NoError(t, err)

	assert.Empty(t, result.RepoURL)
	assert.Contains(t, out.String(), "no repository URL found")
	assert.Contains(t, readFile(t, filepath.Join(repo, release.ChangelogFile)),
		"[Unreleased]: https://github.com/cybersafetyid/featherctl/commits/main")
}

func TestBumpRefusesToMixInUncommittedWork(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)

	writeFile(t, filepath.Join(repo, release.ChangelogFile), changelogWithUnreleased+"\n- pending\n")

	_, err := bump(t, release.Options{Dir: repo, Version: "0.1.0"})
	require.ErrorIs(t, err, release.ErrDirtyChangelog)
	assert.Equal(t, "", gitOut(t, repo, "tag", "--list", "v0.1.0"))

	// --force takes the maintainer's word for it.
	_, err = bump(t, release.Options{Dir: repo, Version: "0.1.0", Force: true})
	require.NoError(t, err)
}

func TestBumpWithoutAnythingToRelease(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, "# Changelog\n\n## [Unreleased]\n")

	_, err := bump(t, release.Options{Dir: repo, Version: "0.1.0"})
	assert.ErrorIs(t, err, changelog.ErrNothingToRelease)
}

func TestBumpDryRunChangesNothing(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)
	head := gitOut(t, repo, "rev-parse", "HEAD")

	var out bytes.Buffer
	result, err := bump(t, release.Options{
		Dir: repo, Version: "0.1.0", Date: "2026-09-22", DryRun: true, Out: &out,
	})
	require.NoError(t, err)

	assert.True(t, result.Promoted, "a dry run still reports what it would promote")
	assert.Equal(t, changelogWithUnreleased, readFile(t, filepath.Join(repo, release.ChangelogFile)))
	assert.Equal(t, head, gitOut(t, repo, "rev-parse", "HEAD"))
	assert.Equal(t, "", gitOut(t, repo, "tag", "--list", "v0.1.0"))
	assert.Contains(t, out.String(), "dry run")
}

func TestBumpNoCommitAndNoTag(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)

	result, err := bump(t, release.Options{
		Dir: repo, Version: "0.1.0", Date: "2026-09-22", NoCommit: true,
	})
	require.NoError(t, err)

	assert.True(t, result.Promoted)
	assert.False(t, result.Committed)
	assert.False(t, result.Tagged)
	assert.Contains(t, readFile(t, filepath.Join(repo, release.ChangelogFile)), "## [0.1.0] - 2026-09-22")
}

func TestBumpRefusesToReuseAnExistingTag(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)
	gitOut(t, repo, "tag", "--annotate", "v0.1.0", "--message", "v0.1.0")

	_, err := bump(t, release.Options{Dir: repo, Version: "0.1.0"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestVerify(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)

	_, err := release.Verify(context.Background(), repo, "v0.1.0")
	require.ErrorIs(t, err, release.ErrUndocumented)
	assert.Contains(t, err.Error(), "## [0.1.0] - YYYY-MM-DD")

	_, err = bump(t, release.Options{Dir: repo, Version: "0.1.0", Date: "2026-09-22"})
	require.NoError(t, err)

	notes, err := release.Verify(context.Background(), repo, "v0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "### Added\n\n- a brand new thing", notes)
}

func TestNotesAndVerifyRejectGarbageVersions(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)

	_, err := release.Notes(context.Background(), repo, "latest")
	assert.Error(t, err)

	_, err = release.Verify(context.Background(), repo, "v1")
	assert.Error(t, err)
}

func TestCheckTags(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, changelogWithUnreleased)

	_, err := bump(t, release.Options{Dir: repo, Version: "0.1.0", Date: "2026-09-22"})
	require.NoError(t, err)

	missing, err := release.CheckTags(context.Background(), repo)
	require.NoError(t, err)
	assert.Empty(t, missing)

	// A tag created by hand, with nothing behind it in the changelog.
	gitOut(t, repo, "tag", "--annotate", "v9.9.9", "--message", "v9.9.9")

	missing, err = release.CheckTags(context.Background(), repo)
	require.NoError(t, err)
	assert.Equal(t, []string{"v9.9.9"}, missing)
}

// bump runs release.Bump with the process output discarded, so the test log
// stays readable even though the tests run in parallel.
func bump(t *testing.T, opts release.Options) (*release.Result, error) {
	t.Helper()
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	return release.Bump(context.Background(), opts)
}

// newRepo creates a temporary git repository whose CHANGELOG.md holds content.
func newRepo(t *testing.T, changelog string) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	dir := t.TempDir()
	gitOut(t, dir, "init", "--initial-branch", "main")
	gitOut(t, dir, "config", "user.name", "feather test")
	gitOut(t, dir, "config", "user.email", "test@example.com")
	gitOut(t, dir, "config", "commit.gpgsign", "false")
	gitOut(t, dir, "config", "remote.origin.url", "https://github.com/cybersafetyid/featherctl.git")

	writeFile(t, filepath.Join(dir, release.ChangelogFile), changelog)
	commitAll(t, dir, "feat: first commit")

	return dir
}

func commitAll(t *testing.T, repo, message string) {
	t.Helper()
	gitOut(t, repo, "add", "--all")
	gitOut(t, repo, "commit", "--message", message)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// gitOut runs git in repo and returns its trimmed output.
func gitOut(t *testing.T, repo string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(t, cmd.Run(), "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}
