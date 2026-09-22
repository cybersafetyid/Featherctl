// Package release implements featherctl's release workflow: it keeps
// `CHANGELOG.md` and the git tags in step.
//
// A release happens in this order:
//
//  1. every change is documented under `[Unreleased]`;
//  2. `feather-release bump X.Y.Z` promotes that section to `## [X.Y.Z]`,
//     re-points the trailing link definitions, commits the changelog and
//     creates the annotated tag;
//  3. pushing the tag triggers .github/workflows/release.yml, which refuses to
//     publish unless the tag has a changelog section and uses that section as
//     the release notes.
//
// Step 2 is the only place that edits the changelog, so the file at a tag
// always describes that tag.
package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cybersafetyid/featherctl/internal/changelog"
)

// ChangelogFile is the file the tooling maintains.
const ChangelogFile = "CHANGELOG.md"

// changelogMode keeps the changelog as readable as the rest of the repository:
// it holds release notes, never secrets.
const changelogMode fs.FileMode = 0o644

// semver matches the versions this project accepts, with or without a leading
// "v" and with optional pre-release and build metadata.
var semver = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// ErrDirtyChangelog means CHANGELOG.md has uncommitted changes that a bump
// would mix into the release commit.
var ErrDirtyChangelog = fmt.Errorf("%s has uncommitted changes", ChangelogFile)

// ErrUndocumented means a version has no changelog section, so nothing would
// describe it on the release page.
var ErrUndocumented = errors.New("version is not documented in the changelog")

// NormalizeVersion accepts "1.2.3" or "v1.2.3" and rejects anything that is not
// a semantic version.
func NormalizeVersion(version string) (string, error) {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	if !semver.MatchString(v) {
		return "", fmt.Errorf("%q is not a semantic version (expected X.Y.Z)", version)
	}
	return v, nil
}

// Options configures a bump.
type Options struct {
	// Dir is the directory to work in. Empty means the process working
	// directory; the repository root is always resolved through git.
	Dir string
	// Version is the version to release, with or without the "v" prefix.
	Version string
	// Date stamps the new changelog section. Empty means today, in UTC.
	Date string
	// RepoURL is the base of the comparison links. Empty means the origin
	// remote, or the module path in go.mod when the clone has no remote.
	RepoURL string
	// DryRun reports what would change without writing, committing or tagging.
	DryRun bool
	// NoCommit leaves the changelog edit uncommitted.
	NoCommit bool
	// NoTag skips creating the annotated tag.
	NoTag bool
	// Push pushes the branch and the tag to origin once they exist.
	Push bool
	// Force allows bumping a dirty CHANGELOG.md.
	Force bool
	// Out receives the human-readable summary. Empty means os.Stdout.
	Out io.Writer
}

// Result describes what a bump did.
type Result struct {
	// Root is the repository root.
	Root string
	// Version is the normalized version that was released.
	Version string
	// Previous is the version the new one is compared against, if any.
	Previous string
	// Notes is the changelog section for Version, ready for a release page.
	Notes string
	// RepoURL is the base of the comparison links. Empty when none was found,
	// in which case the links were left untouched.
	RepoURL string
	// Promoted reports whether [Unreleased] was turned into the new section.
	Promoted bool
	// Committed reports whether the changelog was committed.
	Committed bool
	// Tagged reports whether the annotated tag was created.
	Tagged bool
	// Pushed reports whether the commit and tag were pushed to origin.
	Pushed bool
}

// Bump releases version: it records the changelog section, commits it and tags
// the commit. Nothing is pushed unless Options.Push is set.
func Bump(ctx context.Context, opts Options) (*Result, error) {
	version, err := NormalizeVersion(opts.Version)
	if err != nil {
		return nil, err
	}

	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	root, err := git(ctx, opts.Dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("locate repository root: %w", err)
	}

	changelogPath := filepath.Join(root, ChangelogFile)

	if !opts.Force && !opts.DryRun {
		dirty, err := git(ctx, root, "status", "--porcelain", "--", ChangelogFile)
		if err != nil {
			return nil, err
		}
		if dirty != "" {
			return nil, fmt.Errorf("%w; commit or stash them first, or pass --force", ErrDirtyChangelog)
		}
	}

	raw, err := os.ReadFile(changelogPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ChangelogFile, err)
	}

	doc := changelog.Parse(raw)

	date := opts.Date
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	result := &Result{Root: root, Version: version}

	switch err := doc.Promote(version, date); {
	case err == nil:
		result.Promoted = true
		fmt.Fprintf(out, "  prepared  section [%s] - %s in %s\n", version, date, ChangelogFile)
	case errors.Is(err, changelog.ErrAlreadyReleased):
		fmt.Fprintf(out, "  kept      %s already documents [%s]\n", ChangelogFile, version)
	case errors.Is(err, changelog.ErrNothingToRelease):
		return nil, fmt.Errorf("%w: describe this release under '## [Unreleased]' first", changelog.ErrNothingToRelease)
	case errors.Is(err, changelog.ErrNoSection):
		return nil, err
	default:
		return nil, err
	}

	previous, err := previousVersion(ctx, root, version)
	if err != nil {
		return nil, err
	}
	result.Previous = previous

	repoURL, err := resolveRepoURL(ctx, root, opts.RepoURL)
	if err != nil {
		return nil, err
	}
	result.RepoURL = repoURL

	doc.SyncLinks(version, previous, repoURL)
	if repoURL == "" {
		fmt.Fprintf(out, "  note      no repository URL found: pass --repo-url to update the changelog links\n")
	}

	notes, err := doc.Notes(version)
	if err != nil {
		return nil, err
	}
	result.Notes = notes

	if opts.DryRun {
		fmt.Fprintf(out, "  dry run   would write %s, commit and tag v%s\n", ChangelogFile, version)
		return result, nil
	}

	rendered := doc.Bytes()
	if !bytes.Equal(rendered, raw) {
		if err := os.WriteFile(changelogPath, rendered, changelogMode); err != nil {
			return nil, fmt.Errorf("write %s: %w", ChangelogFile, err)
		}
		fmt.Fprintf(out, "  noted     links now point at v%s\n", version)
	}

	if opts.NoCommit {
		fmt.Fprintf(out, "  skipped   commit (--no-commit); %s is modified\n", ChangelogFile)
		return result, nil
	}

	if _, err := git(ctx, root, "add", "--", ChangelogFile); err != nil {
		return nil, err
	}
	if _, err := git(ctx, root, "commit", "-m", "chore(release): v"+version); err != nil {
		return nil, err
	}
	result.Committed = true
	fmt.Fprintf(out, "  committed chore(release): v%s\n", version)

	if opts.NoTag {
		fmt.Fprintf(out, "  skipped   tag (--no-tag)\n")
		return result, nil
	}

	tag := "v" + version
	existing, err := git(ctx, root, "tag", "--list", tag)
	if err != nil {
		return nil, err
	}
	if existing != "" {
		return nil, fmt.Errorf("tag %s already exists", tag)
	}

	if _, err := git(ctx, root, "tag", "--annotate", tag, "--message", "v"+version); err != nil {
		return nil, err
	}
	result.Tagged = true
	fmt.Fprintf(out, "  tagged    %s\n", tag)

	if !opts.Push {
		return result, nil
	}

	branch, err := git(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	if _, err := git(ctx, root, "push", "origin", branch); err != nil {
		return nil, err
	}
	if _, err := git(ctx, root, "push", "origin", tag); err != nil {
		return nil, err
	}
	result.Pushed = true
	fmt.Fprintf(out, "  pushed    %s and %s to origin\n", branch, tag)

	return result, nil
}

// Notes returns the changelog section for version.
func Notes(ctx context.Context, dir, version string) (string, error) {
	normalized, err := NormalizeVersion(version)
	if err != nil {
		return "", err
	}

	root, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("locate repository root: %w", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ChangelogFile))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", ChangelogFile, err)
	}

	return changelog.Parse(raw).Notes(normalized)
}

// Verify returns the changelog section for version and fails when it is
// missing, which is the check that keeps a release page from shipping with
// empty notes.
func Verify(ctx context.Context, dir, version string) (string, error) {
	normalized, err := NormalizeVersion(version)
	if err != nil {
		return "", err
	}

	notes, err := Notes(ctx, dir, normalized)
	if errors.Is(err, changelog.ErrNoSection) {
		return "", fmt.Errorf(
			"%w: add a '## [%s] - YYYY-MM-DD' section to %s before tagging, or run `feather-release bump %s`",
			ErrUndocumented, normalized, ChangelogFile, normalized,
		)
	}
	return notes, err
}

// CheckTags reports every tag that has no matching changelog section.
func CheckTags(ctx context.Context, dir string) ([]string, error) {
	list, err := git(ctx, dir, "tag", "--list", "v*")
	if err != nil {
		return nil, err
	}

	root, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("locate repository root: %w", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ChangelogFile))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ChangelogFile, err)
	}
	doc := changelog.Parse(raw)

	var missing []string
	for _, tag := range strings.Fields(list) {
		if _, ok := doc.Section(strings.TrimPrefix(tag, "v")); !ok {
			missing = append(missing, tag)
		}
	}
	return missing, nil
}

// resolveRepoURL picks the base URL for the changelog links: the explicit
// flag, then the origin remote, then the module path in go.mod.
func resolveRepoURL(ctx context.Context, root, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	remote, err := originURL(ctx, root)
	if err != nil {
		return "", err
	}
	if remote != "" {
		return remote, nil
	}

	return moduleURL(root), nil
}

// moduleURL reads "module github.com/owner/repo" from go.mod. It is the
// fallback for a clone that has no origin remote, and it is how the released
// repository describes its own URL in go.mod.
func moduleURL(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(raw), "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module ")
		if !ok {
			continue
		}

		path := strings.TrimSpace(rest)
		if i := strings.IndexAny(path, " \t"); i >= 0 {
			path = path[:i]
		}

		host, _, ok := strings.Cut(path, "/")
		if !ok || !strings.Contains(host, ".") {
			return ""
		}
		return "https://" + path
	}
	return ""
}

// previousVersion returns the newest released tag other than version.
func previousVersion(ctx context.Context, root, version string) (string, error) {
	list, err := git(ctx, root, "tag", "--list", "v*", "--sort=-v:refname")
	if err != nil {
		return "", err
	}

	for _, tag := range strings.Fields(list) {
		candidate := strings.TrimPrefix(tag, "v")
		if candidate == version || !semver.MatchString(candidate) {
			continue
		}
		return candidate, nil
	}
	return "", nil
}

// originURL turns the origin remote into a browsable https URL.
func originURL(ctx context.Context, root string) (string, error) {
	remote, err := git(ctx, root, "config", "--get", "remote.origin.url")
	if err != nil || remote == "" {
		return "", nil
	}

	switch {
	case strings.HasPrefix(remote, "git@"):
		host, path, _ := strings.Cut(strings.TrimPrefix(remote, "git@"), ":")
		return "https://" + host + "/" + strings.TrimSuffix(path, ".git"), nil
	case strings.HasPrefix(remote, "ssh://"):
		rest := strings.TrimPrefix(remote, "ssh://")
		if at := strings.Index(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		return "https://" + strings.TrimSuffix(rest, ".git"), nil
	default:
		return strings.TrimSuffix(remote, ".git"), nil
	}
}

// git runs a git command inside dir and returns its trimmed standard output.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return strings.TrimSpace(stdout.String()), nil
}
