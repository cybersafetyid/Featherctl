# Contributing

Thanks for taking the time to help. Bug reports, documentation fixes and pull
requests are all welcome.

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting set up

```bash
git clone https://github.com/cybersafetyid/featherctl
cd featherctl
go build ./...
make test
```

You need Go 1.27.1 or newer (the version in `go.mod`; CI installs exactly that
one from `go-version-file`). `golangci-lint` 2.x and `goreleaser` are only
needed for `make lint` and `make snapshot`.

## Running the tests

```bash
make test-short   # unit tests only; fast
make test         # everything, including the end-to-end suite
make cover        # coverage report for internal/
make e2e          # only the end-to-end suite
```

`make test` builds the CLI, scaffolds a real project in a temporary directory,
generates two features, removes one, and runs `go build ./...` and
`go test ./...` against it. It takes about a minute, and it is the test that
matters most: if it passes, the tool keeps its core promise.

Coverage on `internal/` (excluding the thin `cli` wrappers) is expected to stay
above 80%.

## Branching and commits

- Trunk-based: branch off `main`, open a pull request against `main`, keep the
  branch short-lived.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
  — `feat:`, `fix:`, `docs:`, `chore:`, `test:`, `refactor:`, `ci:` — so the
  history reads well and every release note can be traced back to a commit.
- Keep commits focused. One behaviour change per commit makes review and
  bisecting bearable.
- Add an entry to the `[Unreleased]` section of [CHANGELOG.md](CHANGELOG.md) for
  any user-visible change.

CI runs lint, tests and a build on Linux, macOS and Windows for every pull
request. Everything must be green before merge.

## Project layout

```
cmd/feather/            the CLI users install
cmd/feather-release/    maintainer tooling for cutting a release
internal/cli/           cobra commands — flags in, report out
internal/generator/     orchestration: templates, wiring, reports, doctor
internal/wiring/        dst-based marker injection and removal
internal/config/        feather.yaml schema, defaults, validation
internal/fsutil/        safe writes and conflict detection
internal/changelog/     Keep a Changelog parsing and editing
internal/release/       the release workflow: changelog + commit + tag
templates/              embedded Go templates (embed.FS)
e2e/                    the end-to-end suite
docs/                   user documentation
```

Ground rules:

- `internal/cli` stays thin. No business logic in `RunE`; call into
  `internal/generator` instead.
- Nothing imports `cmd/`.
- `generator`, `wiring`, `config` and `fsutil` never write to standard output
  and never call `os.Exit`. They return values; the CLI renders them.
- Every exported identifier has a doc comment.
- No new dependency without a good reason, raised in the pull request.

## Releasing

Releases are cut with `make bump RELEASE_VERSION=x.y.z`, which records the
version in `CHANGELOG.md`, commits it and creates the annotated tag; pushing the
tag publishes it. A tag whose version `CHANGELOG.md` does not document fails the
release workflow on purpose, and the section in the file becomes the release
notes. The full procedure, including how to write the changelog and how to
release by hand, lives in [docs/releasing.md](docs/releasing.md).

## Adding a template

Templates are Go `text/template` files under `templates/`, embedded into the
binary by `templates/templates.go`.

1. Add the file, for example `templates/feature/audit.go.tmpl`, using
   `{{ .Package }}`, `{{ .Ident }}`, `{{ .Route }}` and the other fields of
   `featureTemplateData` in `internal/generator/feature.go`.
2. Register it in the list the generator iterates — `featureFiles` in
   `internal/generator/feature.go`, or the `specs` slice in
   `internal/generator/project.go`.
3. Regenerate the golden fixtures and review the diff:

   ```bash
   make golden
   git diff internal/generator/testdata
   ```

4. Run `make test`. The end-to-end suite compiles and tests what the template
   produced, so a template that generates code which does not compile fails CI.

If you change an existing template without meaning to, the golden test will fail
with the exact fixture path. Review the diff, and only run `make golden` when
the change is what you intended.

## Adding a marker or a wiring target

Marker handling lives in `internal/wiring`. If you add a new kind of injection
point, add a case there and a unit test in `internal/wiring/injector_test.go`
covering:

- the happy path,
- a missing marker,
- a duplicated marker,
- a marker in an unsupported position,
- running the injection twice (idempotence of the marker itself).

Those five cases are also what `feather remove feature` depends on to be able to
reverse an injection, so test removal alongside it.

## Agent skills

`.agents/skills/` holds the Go development skills used while working on this
repository (installed with `npx skills add`). They are checked in so every
contributor's tooling starts from the same conventions: Go CLI and cobra
structure, testing, error handling, naming, documentation and lint
configuration. They are inert — nothing under `.agents/` is part of the module,
the build or the released binary, and `go list ./...` ignores it.

## Reporting a bug

Please include:

- the output of `feather --version`,
- your OS and Go version,
- the smallest `feather.yaml` and file layout that reproduces it,
- what you expected and what happened instead.

The bug report template in `.github/ISSUE_TEMPLATE/` asks for exactly that.
