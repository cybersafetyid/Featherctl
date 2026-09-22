// Package wiring injects generated code into existing Go files by locating
// marker comments in a decorated syntax tree (github.com/dave/dst).
//
// Using dst instead of go/ast means the parts of the file featherctl does not
// touch — comments, blank lines, grouping — come out of a generation run
// byte-for-byte identical. That property is what makes it safe to run
// `feather new feature` on a project a human has been editing by hand.
package wiring

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"golang.org/x/tools/imports"
)

// Injection describes a single marker-based edit of a Go file.
type Injection struct {
	// Path is the file being edited. It is only used for goimports resolution
	// and error messages, but it must be a real path inside the target module
	// for import resolution to work.
	Path string
	// Marker is the exact marker comment to insert code at. Surrounding
	// whitespace is ignored.
	Marker string
	// Code is Go source without the surrounding package or function. It is
	// treated as statements when the marker sits in a statement list and as
	// struct fields when the marker sits in a field list.
	Code string
	// Imports are import paths that must exist in the file afterwards.
	Imports []string
}

// Removal describes the removal of code that was previously injected.
type Removal struct {
	// Path is the file being edited.
	Path string
	// Lines are the exact source lines to delete. Each line must appear exactly
	// once in the file; anything else aborts the removal.
	Lines []string
}

// MarkerNotFoundError reports a file that no longer contains the marker
// featherctl needs to inject at.
type MarkerNotFoundError struct {
	// Path is the file that was searched.
	Path string
	// Marker is the comment that could not be found.
	Marker string
}

// Error implements the error interface.
func (e *MarkerNotFoundError) Error() string {
	return fmt.Sprintf("marker %s not found in %s", strconv.Quote(e.Marker), displayPath(e.Path))
}

// DuplicateMarkerError reports a file that contains the marker comment more
// than once, which makes the insertion point ambiguous.
//
// featherctl refuses to guess: it aborts and asks the user to remove the
// duplicate rather than injecting into a random one of the two locations.
type DuplicateMarkerError struct {
	// Path is the file that contains the duplicated marker.
	Path string
	// Marker is the duplicated comment.
	Marker string
	// Count is the number of occurrences.
	Count int
}

// Error implements the error interface.
func (e *DuplicateMarkerError) Error() string {
	return fmt.Sprintf("%s contains %d copies of marker %s; remove the duplicate before generating",
		displayPath(e.Path), e.Count, strconv.Quote(e.Marker))
}

// MarkerPositionError reports a marker that exists but sits somewhere
// featherctl cannot safely insert code.
type MarkerPositionError struct {
	// Path is the file that contains the marker.
	Path string
	// Marker is the comment that was found.
	Marker string
	// Position describes where the marker actually is.
	Position string
}

// Error implements the error interface.
func (e *MarkerPositionError) Error() string {
	return fmt.Sprintf("marker %s in %s is not in a supported position (%s)",
		strconv.Quote(e.Marker), displayPath(e.Path), e.Position)
}

// LineNotFoundError reports an expected line that is missing during removal.
type LineNotFoundError struct {
	// Path is the file that should have contained the line.
	Path string
	// Line is the missing source line.
	Line string
}

// Error implements the error interface.
func (e *LineNotFoundError) Error() string {
	return fmt.Sprintf("%s does not contain %s", displayPath(e.Path), strconv.Quote(strings.TrimSpace(e.Line)))
}

// AmbiguousLineError reports an expected line that appears more than once,
// which makes a safe removal impossible.
type AmbiguousLineError struct {
	// Path is the file that contains the duplicated line.
	Path string
	// Line is the duplicated source line.
	Line string
	// Count is the number of occurrences.
	Count int
}

// Error implements the error interface.
func (e *AmbiguousLineError) Error() string {
	return fmt.Sprintf("%s contains %d copies of %s, refusing to guess which one to remove",
		displayPath(e.Path), e.Count, strconv.Quote(strings.TrimSpace(e.Line)))
}

// ErrEmptyMarker is returned when no marker was configured.
var ErrEmptyMarker = errors.New("wiring: marker must not be empty")

// Inject returns src with inj.Code inserted at inj.Marker and every import in
// inj.Imports present.
//
// The returned source is formatted with goimports, so callers can write it
// straight to disk.
func Inject(src []byte, inj Injection) ([]byte, error) {
	if count := CountMarker(src, inj.Marker); count > 1 {
		return nil, &DuplicateMarkerError{Path: inj.Path, Marker: strings.TrimSpace(inj.Marker), Count: count}
	}

	file, err := parse(inj.Path, src)
	if err != nil {
		return nil, err
	}

	anchor, err := findAnchor(file, inj)
	if err != nil {
		return nil, err
	}
	if err := anchor.insert(inj.Code); err != nil {
		return nil, err
	}
	for _, path := range inj.Imports {
		addImport(file, path)
	}

	return render(inj.Path, file)
}

// Remove deletes every line in rem.Lines from src exactly once, formats the
// result and drops imports that are no longer used.
//
// If any line is missing or ambiguous, removal aborts and the caller must fall
// back to manual editing: guessing would risk corrupting an unrelated line.
func Remove(src []byte, rem Removal) ([]byte, error) {
	if _, err := parse(rem.Path, src); err != nil {
		return nil, err
	}

	lines := splitLines(src)
	doomed := make(map[int]bool, len(rem.Lines))

	for _, want := range rem.Lines {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}

		var matches []int
		for i, line := range lines {
			if strings.TrimSpace(line) == want {
				matches = append(matches, i)
			}
		}

		switch len(matches) {
		case 0:
			return nil, &LineNotFoundError{Path: rem.Path, Line: want}
		case 1:
			doomed[matches[0]] = true
		default:
			return nil, &AmbiguousLineError{Path: rem.Path, Line: want, Count: len(matches)}
		}
	}

	kept := make([]string, 0, len(lines))
	for i, line := range lines {
		if doomed[i] {
			continue
		}
		kept = append(kept, line)
	}

	return FormatSource(rem.Path, []byte(strings.Join(kept, "\n")))
}

// ContainsMarker reports whether src contains a line that is exactly the
// marker comment. It is used by `feather doctor` to spot projects where a
// human deleted an injection point.
func ContainsMarker(src []byte, marker string) bool {
	return CountMarker(src, marker) > 0
}

// CountMarker counts how many times src contains the marker comment.
func CountMarker(src []byte, marker string) int {
	want := strings.TrimSpace(marker)
	if want == "" {
		return 0
	}

	count := 0
	for _, line := range splitLines(src) {
		if strings.TrimSpace(line) == want {
			count++
		}
	}
	return count
}

// FormatSource formats generated Go source with goimports, falling back to
// gofmt when the file cannot be resolved as part of a module.
func FormatSource(path string, src []byte) ([]byte, error) {
	if path != "" && strings.HasSuffix(path, ".go") {
		if out, err := imports.Process(path, src, nil); err == nil {
			return out, nil
		}
	}

	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("wiring: format %s: %w", displayPath(path), err)
	}
	return out, nil
}

// parse decorates a Go source file.
func parse(path string, src []byte) (*dst.File, error) {
	file, err := decorator.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("wiring: parse %s: %w", displayPath(path), err)
	}
	return file, nil
}

// render prints a decorated tree back to source and formats it.
func render(path string, file *dst.File) ([]byte, error) {
	var buf bytes.Buffer
	if err := decorator.Fprint(&buf, file); err != nil {
		return nil, fmt.Errorf("wiring: print %s: %w", displayPath(path), err)
	}
	return FormatSource(path, buf.Bytes())
}

// splitLines splits source into lines without normalising line endings.
func splitLines(src []byte) []string {
	return strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
}

// displayPath shortens a path for error messages.
func displayPath(path string) string {
	if path == "" {
		return "<source>"
	}
	return filepath.ToSlash(path)
}
