package changelog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/changelog"
)

const sample = `# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- something new

## [0.1.0] - 2026-08-01

### Added

- the first thing

[Unreleased]: https://github.com/cybersafetyid/featherctl/commits/main
[0.1.0]: https://github.com/cybersafetyid/featherctl/releases/tag/v0.1.0
`

func TestParseSections(t *testing.T) {
	t.Parallel()

	doc := changelog.Parse([]byte(sample))
	sections := doc.Sections()
	require.Len(t, sections, 2)

	assert.Equal(t, "Unreleased", sections[0].Version)
	assert.Empty(t, sections[0].Meta)
	assert.Equal(t, "### Added\n\n- something new", sections[0].Body)

	assert.Equal(t, "0.1.0", sections[1].Version)
	assert.Equal(t, " - 2026-08-01", sections[1].Meta)
	assert.Equal(t, "### Added\n\n- the first thing", sections[1].Body)
}

func TestParseHeadings(t *testing.T) {
	t.Parallel()

	doc := changelog.Parse([]byte("## [1.2.3] - 2026-01-02\n\nbody\n\n## 1.2.4\n\nbody\n\n## [1.2.5]\n"))

	got := map[string]string{}
	for _, s := range doc.Sections() {
		got[s.Version] = s.Meta
	}

	assert.Equal(t, " - 2026-01-02", got["1.2.3"])
	assert.Equal(t, "", got["1.2.4"])
	assert.Equal(t, "", got["1.2.5"])
}

func TestParsePreservesDocument(t *testing.T) {
	t.Parallel()

	for name, src := range map[string]string{
		"sample":      sample,
		"crlf":        strings.ReplaceAll(sample, "\n", "\r\n"),
		"no trailing": strings.TrimSuffix(sample, "\n"),
		"no links":    "## [Unreleased]\n\n- a change\n",
		"empty":       "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := strings.ReplaceAll(src, "\r\n", "\n")
			require.Equal(t, want, string(changelog.Parse([]byte(src)).Bytes()))
		})
	}
}

func TestParseRepositoryChangelog(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "CHANGELOG.md")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	doc := changelog.Parse(raw)
	require.Equal(t, string(raw), string(doc.Bytes()), "the parser must not rewrite this repository's changelog")

	_, ok := doc.Section(changelog.Unreleased)
	require.True(t, ok, "this repository documents its work under [Unreleased]")
}

func TestNotes(t *testing.T) {
	t.Parallel()

	doc := changelog.Parse([]byte(sample))

	notes, err := doc.Notes("0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "### Added\n\n- the first thing", notes)

	_, err = doc.Notes("9.9.9")
	assert.ErrorIs(t, err, changelog.ErrNoSection)
}

func TestPromote(t *testing.T) {
	t.Parallel()

	doc := changelog.Parse([]byte(sample))
	require.NoError(t, doc.Promote("0.2.0", "2026-09-22"))

	assert.Equal(t, `# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

## [0.2.0] - 2026-09-22

### Added

- something new

## [0.1.0] - 2026-08-01

### Added

- the first thing

[Unreleased]: https://github.com/cybersafetyid/featherctl/commits/main
[0.1.0]: https://github.com/cybersafetyid/featherctl/releases/tag/v0.1.0
`, string(doc.Bytes()))

	sections := doc.Sections()
	require.Len(t, sections, 3)
	assert.Equal(t, "Unreleased", sections[0].Version)
	assert.Empty(t, sections[0].Body, "the new [Unreleased] section starts empty")
	assert.Equal(t, "0.2.0", sections[1].Version)
	assert.Equal(t, "### Added\n\n- something new", sections[1].Body)
}

func TestPromoteOpensAFreshUnreleasedSection(t *testing.T) {
	t.Parallel()

	doc := changelog.Parse([]byte(sample))
	require.NoError(t, doc.Promote("0.2.0", "2026-09-22"))
	assert.Equal(t, []string{"Unreleased", "0.2.0", "0.1.0"}, versions(doc))

	// The new [Unreleased] is empty until a human documents the next change, so
	// promoting again immediately has nothing to release.
	assert.ErrorIs(t, doc.Promote("0.2.1", "2026-10-01"), changelog.ErrNothingToRelease)
}

func TestPromoteKeepsOlderSections(t *testing.T) {
	t.Parallel()

	doc := changelog.Parse([]byte(sample))
	require.NoError(t, doc.Promote("0.2.0", "2026-09-22"))

	// The next cycle: someone documents a fix under [Unreleased].
	next := changelog.Parse([]byte(strings.Replace(
		string(doc.Bytes()),
		"## [Unreleased]\n\n## [0.2.0]",
		"## [Unreleased]\n\n### Fixed\n\n- a fix\n\n## [0.2.0]",
		1,
	)))
	require.NoError(t, next.Promote("0.2.1", "2026-10-01"))

	assert.Equal(t, []string{"Unreleased", "0.2.1", "0.2.0", "0.1.0"}, versions(next))
	assert.Contains(t, string(next.Bytes()), "## [0.2.0] - 2026-09-22")
	assert.Contains(t, string(next.Bytes()), "- a fix")

	previous, err := next.Notes("0.2.0")
	require.NoError(t, err)
	assert.Equal(t, "### Added\n\n- something new", previous)
}

func TestPromoteErrors(t *testing.T) {
	t.Parallel()

	t.Run("already released", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte(sample))
		assert.ErrorIs(t, doc.Promote("0.1.0", "2026-09-22"), changelog.ErrAlreadyReleased)
	})

	t.Run("nothing to release", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte("## [Unreleased]\n\n## [0.1.0] - 2026-01-01\n\n- a thing\n"))
		assert.ErrorIs(t, doc.Promote("0.2.0", "2026-09-22"), changelog.ErrNothingToRelease)
	})

	t.Run("no unreleased section", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte("## [0.1.0] - 2026-01-01\n\n- a thing\n"))
		assert.ErrorIs(t, doc.Promote("0.2.0", "2026-09-22"), changelog.ErrNoSection)
	})

	t.Run("wrong heading level is ignored", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte("# Changelog\n\n### [Unreleased]\n\n- a thing\n"))
		assert.ErrorIs(t, doc.Promote("0.2.0", "2026-09-22"), changelog.ErrNoSection)
	})
}

func TestSyncLinks(t *testing.T) {
	t.Parallel()

	const base = "https://github.com/cybersafetyid/featherctl"

	t.Run("adds both links to a document without any", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte("## [Unreleased]\n\n- a thing\n"))
		doc.SyncLinks("0.1.0", "", base)

		assert.Equal(t, []changelog.Link{
			{Label: "Unreleased", URL: base + "/compare/v0.1.0...HEAD"},
			{Label: "0.1.0", URL: base + "/releases/tag/v0.1.0"},
		}, doc.Links())
	})

	t.Run("compares against the previous version", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte(sample))
		doc.SyncLinks("0.2.0", "0.1.0", base)

		assert.Equal(t, []changelog.Link{
			{Label: "Unreleased", URL: base + "/compare/v0.2.0...HEAD"},
			{Label: "0.2.0", URL: base + "/compare/v0.1.0...v0.2.0"},
			{Label: "0.1.0", URL: base + "/releases/tag/v0.1.0"},
		}, doc.Links())
	})

	t.Run("is idempotent", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte(sample))
		doc.SyncLinks("0.2.0", "0.1.0", base)
		once := string(doc.Bytes())

		doc.SyncLinks("0.2.0", "0.1.0", base)
		assert.Equal(t, once, string(doc.Bytes()))
	})

	t.Run("tolerates a trailing slash and .git", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte(sample))
		doc.SyncLinks("0.2.0", "0.1.0", base+".git/")

		assert.Equal(t, base+"/compare/v0.2.0...HEAD", doc.Links()[0].URL)
	})

	t.Run("does nothing without a repository url", func(t *testing.T) {
		t.Parallel()
		doc := changelog.Parse([]byte(sample))
		before := string(doc.Bytes())

		doc.SyncLinks("0.2.0", "0.1.0", "  ")
		assert.Equal(t, before, string(doc.Bytes()))
	})
}

func TestSectionTitle(t *testing.T) {
	t.Parallel()

	doc := changelog.Parse([]byte(sample))
	section, ok := doc.Section("0.1.0")
	require.True(t, ok)
	assert.Equal(t, "[0.1.0] - 2026-08-01", section.Title())
}

func TestNotesIncludesEverythingUnderTheHeading(t *testing.T) {
	t.Parallel()

	doc := changelog.Parse([]byte(sample))
	notes, err := doc.Notes(changelog.Unreleased)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(notes, "### Added"))
	assert.Contains(t, notes, "- something new")
	assert.NotContains(t, notes, "## [", "the release notes must not carry the next section")
}

func versions(doc *changelog.Document) []string {
	out := make([]string, 0, len(doc.Sections()))
	for _, s := range doc.Sections() {
		out = append(out, s.Version)
	}
	return out
}
