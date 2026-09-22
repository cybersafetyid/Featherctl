// Package changelog parses, edits and renders `CHANGELOG.md` files that follow
// the [Keep a Changelog] convention.
//
// The release tooling uses it to guarantee one invariant: every released tag
// has a matching version section in `CHANGELOG.md`. `feather-release bump`
// promotes the `[Unreleased]` section into the section for the version being
// tagged, so the file on the tag — and therefore the release page built from
// it — always describes what shipped.
//
// The parser is intentionally small. A section is a line starting with `## ` at
// column zero; trailing definitions of the form `[label]: url` are kept
// separately so they can be re-pointed at the new tag.
//
// [Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
package changelog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Errors reported by this package. Callers are expected to match them with
// errors.Is.
var (
	// ErrNoSection means the document has no section for the requested version.
	ErrNoSection = errors.New("changelog section not found")
	// ErrNothingToRelease means `[Unreleased]` has no content to promote.
	ErrNothingToRelease = errors.New("[Unreleased] section is empty")
	// ErrAlreadyReleased means the requested version already has a section.
	ErrAlreadyReleased = errors.New("version already documented in the changelog")
)

// Unreleased is the label Keep a Changelog reserves for pending changes.
const Unreleased = "Unreleased"

// linkDef matches a trailing markdown link definition such as
// `[Unreleased]: https://example.com/compare/v1.0.0...HEAD`.
var linkDef = regexp.MustCompile(`^\[([^\]]+)\]:[ \t]*(\S+)[ \t]*$`)

// Section is one `## ` section of the document.
type Section struct {
	// Version is the section's label, for example "1.2.3" or "Unreleased".
	Version string
	// Meta is what follows the label, for example " - 2026-09-22".
	Meta string
	// Body is the markdown under the heading, with leading and trailing blank
	// lines removed.
	Body string
}

// Title renders the heading of the section, without the leading "## ".
func (s Section) Title() string { return "[" + s.Version + "]" + s.Meta }

// Link is a trailing markdown link definition such as
// `[1.2.3]: https://example.com/compare/v1.2.2...v1.2.3`.
type Link struct {
	Label string
	URL   string
}

// Document is a parsed CHANGELOG.md.
//
// Rendering a parsed document that was not modified reproduces the input byte
// for byte, which is what makes it safe to run the release tooling on a
// changelog that a human maintains by hand. Line endings are the one exception:
// CRLF input is normalised to LF, as the repository's .gitattributes requires.
type Document struct {
	head   []string // every line before the trailing link definitions
	links  []Link
	spans  []span
	ending string // "\n" when the source ended with a newline
}

// span records where a section lives inside head.
type span struct {
	section   Section
	headLine  int // index of the "## " line
	bodyStart int // index of the first line after the heading
	bodyEnd   int // index of the next "## " line, or len(head)
	bodyHead  int // bodyStart without leading blank lines
	bodyTail  int // bodyEnd without trailing blank lines
}

// Parse reads a CHANGELOG.md.
//
// CRLF input is accepted; rendering always emits LF, matching what the
// repository's .gitattributes enforces.
func Parse(src []byte) *Document {
	text := strings.ReplaceAll(string(src), "\r\n", "\n")

	ending := ""
	if strings.HasSuffix(text, "\n") {
		ending = "\n"
	}

	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	// Walk back over the trailing run of blank lines and link definitions, then
	// start the link block at the first definition so that head keeps the blank
	// separator line: that is what makes rendering an untouched document
	// reproduce it byte for byte.
	runStart := len(lines)
	for runStart > 0 {
		line := lines[runStart-1]
		if strings.TrimSpace(line) == "" || linkDef.MatchString(line) {
			runStart--
			continue
		}
		break
	}

	linksStart := runStart
	for linksStart < len(lines) && strings.TrimSpace(lines[linksStart]) == "" {
		linksStart++
	}

	var links []Link
	for _, line := range lines[linksStart:] {
		if m := linkDef.FindStringSubmatch(line); m != nil {
			links = append(links, Link{Label: m[1], URL: m[2]})
		}
	}

	doc := &Document{head: lines[:linksStart], links: links, ending: ending}
	doc.reindex()
	return doc
}

// Bytes renders the document.
func (d *Document) Bytes() []byte {
	lines := make([]string, 0, len(d.head)+len(d.links))
	lines = append(lines, d.head...)
	for _, l := range d.links {
		lines = append(lines, fmt.Sprintf("[%s]: %s", l.Label, l.URL))
	}
	return []byte(strings.Join(lines, "\n") + d.ending)
}

// Sections returns every section in document order.
func (d *Document) Sections() []Section {
	out := make([]Section, 0, len(d.spans))
	for _, s := range d.spans {
		out = append(out, s.section)
	}
	return out
}

// Links returns the trailing link definitions in document order.
func (d *Document) Links() []Link { return append([]Link(nil), d.links...) }

// Section returns the section for version and whether it exists.
func (d *Document) Section(version string) (Section, bool) {
	for _, s := range d.spans {
		if s.section.Version == version {
			return s.section, true
		}
	}
	return Section{}, false
}

// Notes returns the body of the section for version, which is what a release
// page should display. It returns ErrNoSection when the version is missing.
func (d *Document) Notes(version string) (string, error) {
	section, ok := d.Section(version)
	if !ok {
		return "", fmt.Errorf("%w: no section for %q", ErrNoSection, version)
	}
	return section.Body, nil
}

// Promote turns the `[Unreleased]` section into the section for version,
// stamped with date, and opens a fresh empty `[Unreleased]` section above it.
//
// It returns ErrAlreadyReleased when version already has a section — the
// caller can then release the file as it stands — and ErrNothingToRelease when
// there is nothing pending to document.
func (d *Document) Promote(version, date string) error {
	if _, ok := d.Section(version); ok {
		return fmt.Errorf("%w: %q", ErrAlreadyReleased, version)
	}

	current, ok := d.find(Unreleased)
	if !ok {
		return fmt.Errorf("%w: no [%s] section", ErrNoSection, Unreleased)
	}
	if current.section.Body == "" {
		return ErrNothingToRelease
	}

	heading := "## [" + version + "]"
	if date != "" {
		heading += " - " + date
	}

	body := make([]string, 0, current.bodyTail-current.bodyHead)
	body = append(body, d.head[current.bodyHead:current.bodyTail]...)

	promoted := make([]string, 0, len(body)+4)
	promoted = append(promoted, "## ["+Unreleased+"]", "")
	promoted = append(promoted, heading, "")
	promoted = append(promoted, body...)
	promoted = append(promoted, "")

	d.head = splice(d.head, current.headLine, current.bodyEnd, promoted)
	d.reindex()
	return nil
}

// SyncLinks points `[Unreleased]` at the comparison range that starts at
// version and records where version can be compared from. previous may be
// empty, in which case the link points at the tag itself.
//
// repoURL is the repository's web URL, for example
// "https://github.com/cybersafetyid/featherctl".
func (d *Document) SyncLinks(version, previous, repoURL string) {
	base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(repoURL), "/"), ".git")
	if base == "" {
		return
	}

	unreleasedURL := fmt.Sprintf("%s/compare/v%s...HEAD", base, version)

	versionURL := fmt.Sprintf("%s/releases/tag/v%s", base, version)
	if previous != "" {
		versionURL = fmt.Sprintf("%s/compare/v%s...v%s", base, previous, version)
	}

	unreleasedAt := -1
	versionAt := -1
	for i, l := range d.links {
		switch l.Label {
		case Unreleased:
			unreleasedAt = i
		case version:
			versionAt = i
		}
	}

	if unreleasedAt >= 0 {
		d.links[unreleasedAt].URL = unreleasedURL
	} else {
		d.links = append([]Link{{Label: Unreleased, URL: unreleasedURL}}, d.links...)
		unreleasedAt = 0
		if versionAt >= 0 {
			versionAt++
		}
	}

	if versionAt >= 0 {
		d.links[versionAt].URL = versionURL
		return
	}

	insertAt := unreleasedAt + 1
	next := make([]Link, 0, len(d.links)+1)
	next = append(next, d.links[:insertAt]...)
	next = append(next, Link{Label: version, URL: versionURL})
	next = append(next, d.links[insertAt:]...)
	d.links = next
}

// find returns the span for version.
func (d *Document) find(version string) (span, bool) {
	for _, s := range d.spans {
		if s.section.Version == version {
			return s, true
		}
	}
	return span{}, false
}

// reindex rebuilds the section index after head changed.
func (d *Document) reindex() {
	d.spans = d.spans[:0]

	for i := 0; i < len(d.head); i++ {
		if !strings.HasPrefix(d.head[i], "## ") {
			continue
		}
		version, meta := splitHeading(d.head[i])

		end := len(d.head)
		for j := i + 1; j < len(d.head); j++ {
			if strings.HasPrefix(d.head[j], "## ") {
				end = j
				break
			}
		}

		head, tail := i+1, end
		for head < tail && strings.TrimSpace(d.head[head]) == "" {
			head++
		}
		for tail > head && strings.TrimSpace(d.head[tail-1]) == "" {
			tail--
		}

		d.spans = append(d.spans, span{
			section: Section{
				Version: version,
				Meta:    meta,
				Body:    strings.Join(d.head[head:tail], "\n"),
			},
			headLine:  i,
			bodyStart: i + 1,
			bodyEnd:   end,
			bodyHead:  head,
			bodyTail:  tail,
		})

		i = end - 1
	}
}

// splitHeading splits "## [1.2.3] - 2026-09-22" into "1.2.3" and
// " - 2026-09-22". It also accepts the unbracketed "## 1.2.3" form.
func splitHeading(line string) (version, meta string) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "## "))

	if strings.HasPrefix(rest, "[") {
		if end := strings.Index(rest, "]"); end > 0 {
			return rest[1:end], rest[end+1:]
		}
		return rest, ""
	}

	if i := strings.IndexAny(rest, " \t"); i > 0 {
		return rest[:i], rest[i:]
	}
	return rest, ""
}

// splice replaces lines[start:end] with replacement.
func splice(lines []string, start, end int, replacement []string) []string {
	out := make([]string, 0, len(lines)-(end-start)+len(replacement))
	out = append(out, lines[:start]...)
	out = append(out, replacement...)
	return append(out, lines[end:]...)
}
