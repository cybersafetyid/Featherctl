# Releasing

featherctl has one rule that makes releases boring:

> **A tag must already be documented in `CHANGELOG.md`.**

Because of that rule the release page for `vX.Y.Z`, the `CHANGELOG.md` inside the
release archive and the file at the tagged commit all say the same thing — and
none of them can be empty.

## The short version

```bash
# 1. Land your work on main, described under "## [Unreleased]".
# 2. Release it.
make bump RELEASE_VERSION=0.2.0

# 3. Publish: the tag triggers .github/workflows/release.yml.
git push origin HEAD --follow-tags
```

`make bump` is a thin wrapper around `go run ./cmd/feather-release bump`, so the
same thing works from any shell:

```bash
go run ./cmd/feather-release bump 0.2.0
go run ./cmd/feather-release bump 0.2.0 --push        # also push the commit and tag
go run ./cmd/feather-release bump v0.2.0 --dry-run    # show what would change
```

The `v` prefix is optional everywhere; the tag is always `vX.Y.Z`.

## What `bump` does

1. **Checks the working tree.** `CHANGELOG.md` must be committed, so a release
   commit never sweeps up unrelated edits. Override with `--force`.
2. **Promotes `[Unreleased]`** into `## [X.Y.Z] - YYYY-MM-DD` and opens a fresh,
   empty `[Unreleased]` above it. If `CHANGELOG.md` already documents the
   version — because you wrote the section by hand — it is left alone.
3. **Re-points the trailing link definitions**: `[Unreleased]` becomes
   `…/compare/vX.Y.Z...HEAD` and `[X.Y.Z]` becomes
   `…/compare/vPREVIOUS...vX.Y.Z` (or `…/releases/tag/vX.Y.Z` for the first
   release). The base URL is the origin remote, or the `module` path in
   `go.mod` when the clone has no remote; `--repo-url` overrides both.
4. **Commits** just `CHANGELOG.md`, as `chore(release): vX.Y.Z`.
5. **Creates the annotated tag** `vX.Y.Z`.

Nothing is pushed unless you ask for it with `--push`.

A dry run is safe and shows every step:

```console
$ go run ./cmd/feather-release bump 0.2.0 --dry-run
Releasing featherctl v0.2.0
  prepared  section [0.2.0] - 2026-09-22 in CHANGELOG.md
  dry run   would write CHANGELOG.md, commit and tag v0.2.0

Dry run: nothing was written, committed or tagged.
```

## What the workflow does

`.github/workflows/release.yml` runs when a `v*` tag is pushed:

1. `go test ./...`
2. `feather-release verify "$GITHUB_REF_NAME"` — **fails the release** with

   ```text
   version is not documented in the changelog: add a '## [0.2.0] - YYYY-MM-DD'
   section to CHANGELOG.md before tagging, or run `feather-release bump 0.2.0`
   ```

   if the tag has no section;
3. `feather-release notes "$GITHUB_REF_NAME" > release-notes.md`;
4. GoReleaser with `--release-notes=release-notes.md`, which replaces its own
   generated changelog with that curated section.

## Writing the changelog

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
While you work, add a bullet under the matching heading of `[Unreleased]`:

```markdown
## [Unreleased]

### Added

- `feather new feature <name>` now …
```

Use `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed` or
`### Security`. Write for the person upgrading, not for the reviewer: "`doctor`
now fails when a marker is missing" beats "fix marker check".

A version with an empty `[Unreleased]` is not a release — `bump` says so
instead of tagging an empty section.

## Checking a repository

```bash
go run ./cmd/feather-release check      # every v* tag has a section?
go run ./cmd/feather-release notes v0.1.0
go run ./cmd/feather-release verify v0.1.0
```

`check` is the guard for a tag that was created by hand:

```console
$ go run ./cmd/feather-release check
feather-release: CHANGELOG.md has no section for: v0.3.0
```

## Releasing by hand

You can skip the tool — the workflow only cares about the result:

1. write `## [X.Y.Z] - YYYY-MM-DD` in `CHANGELOG.md`, with the release notes
   under it, and commit it;
2. `git tag --annotate vX.Y.Z --message "vX.Y.Z"`;
3. `git push origin HEAD --follow-tags`.

The workflow's `verify` step is what makes this safe: forget step 1 and the
release fails before anything is published.

## Version numbers

Releases follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html):
breaking changes bump the major, new features the minor, fixes the patch. A
release candidate is a pre-release version — `bump 1.0.0-rc.1` works, and
GoReleaser marks it as a pre-release automatically.

The version a binary reports comes from the git tag, injected by GoReleaser
with `-X main.version=…` and by `make build` with `git describe`. No source file
hardcodes it, so a bump only ever touches `CHANGELOG.md` and the tags.
