# featherctl

**Add a feature. Not a headache.**

Every Go project generator scaffolds a project *once*, at the very beginning.
They do not help with the thing you actually do every week: adding a new feature
to a codebase that already exists. `feather` generates one complete vertical
slice — model, repository, service, handler and tests — and wires it into your
router and dependency container. No manual edits, no "now go and update five
files so it compiles".

<!-- TODO: add demo gif -->

```console
$ feather new feature order
✔ Created internal/features/order/model.go
✔ Created internal/features/order/repository.go
✔ Created internal/features/order/service.go
✔ Created internal/features/order/handler.go
✔ Created internal/features/order/handler_test.go
✔ Created internal/features/order/service_test.go
✔ Wired routes into internal/platform/router/router.go
✔ Wired container into internal/platform/container/container.go

Feature "order" is ready. Run `go build ./...` to verify.
```

## Install

```bash
go install github.com/cybersafetyid/featherctl/cmd/feather@latest
```

featherctl itself needs Go 1.27.1 or newer — it ships Go templates and rewrites
Go source with `dave/dst`, and it is built and tested against the current
release. The projects it *generates* stay compatible with Go 1.22, because they
only use the standard library's method-aware `net/http` router.

## Quick start

```bash
# 1. Scaffold a project. `--module` is required: feather never guesses an
#    import path for you.
feather init demo --module github.com/you/demo
cd demo

# 2. Add a feature. Everything it needs is created and wired in.
feather new feature order

# 3. It compiles, and so do the tests that were generated with it.
go build ./...
go test ./...

# 4. Add another one. Both are wired, and neither one conflicts.
feather new feature user-profile
go build ./...
```

Preview anything before it touches your disk:

```bash
feather new feature invoice --dry-run
```

Undo a feature — the files, the routes and the container entries all go away:

```bash
feather remove feature invoice
```

Check that a project is still ready for generation (it exits non-zero when a
check fails, so it works as a CI gate):

```bash
feather doctor
```

## What gets generated

```
internal/features/order/
├── model.go          request, response and domain types
├── repository.go     Repository interface + in-memory implementation
├── service.go        business rules, depends on the repository interface
├── handler.go        HTTP handler, depends on the service
├── handler_test.go   table-driven handler tests, exercised through the mux
└── service_test.go   table-driven service tests using a stub repository
```

And these two files get one line each, at their marker comment:

```go
// internal/platform/router/router.go
func New(c *container.Container) *http.ServeMux {
	mux := http.NewServeMux()

	c.Order.RegisterRoutes(mux)

	// feather:register-routes (do not remove this comment)

	return mux
}
```

```go
// internal/platform/container/container.go
type Container struct {
	// Order handles HTTP requests for the order feature.
	Order *order.Handler

	// feather:container-fields (do not remove this comment)
	Logger *slog.Logger
}
```

## How it stays safe

- **It never guesses.** A missing marker, an ambiguous marker or a feature that
  already exists aborts the run with an explanation instead of a half-applied
  edit.
- **It never touches what it does not own.** Files are parsed with
  [`dave/dst`](https://github.com/dave/dst), so the comments, blank lines and
  grouping you wrote by hand come out of a generation run byte-for-byte
  identical.
- **It never silently overwrites.** `--force` is required to replace anything
  that already exists, and `--dry-run` shows the whole plan first.
- **Generated code compiles.** `--validate` runs `go build ./...` for you, and
  the project's own test suite proves the same thing on every push.

## Documentation

- [Getting started](docs/getting-started.md) — install, scaffold, add a feature,
  use it in CI.
- [Architecture](docs/architecture.md) — what vertical slice architecture is,
  why feather generates slices this way, and how it relates to whole-project
  scaffolds like go-blueprint.
- [Configuration](docs/configuration.md) — every option in `feather.yaml`,
  including how to move the markers.
- [Releasing](docs/releasing.md) — how a version gets into `CHANGELOG.md`, and
  how the tag, the release notes and the archive stay in step.

## Commands

| Command | What it does |
| --- | --- |
| `feather init <name>` | Scaffold a new vertical-slice project (`--module`, `--force`, `--dry-run`, `--validate`) |
| `feather new feature <name>` | Generate one feature slice and wire it in (`--force`, `--dry-run`, `--validate`) |
| `feather remove feature <name>` | Delete a feature and un-wire it (`--dry-run`) |
| `feather doctor [path]` | Check config, markers and compilation (`--skip-build`) |

Feature names may be written as `order`, `user-profile` or `userProfile`; all
three produce the same package, identifiers and route.

## Requirements feather places on your project

- Go 1.22+ and `net/http` — the generated code uses the standard library's
  method-aware `ServeMux` and adds no third-party dependency to your project.
  (A newer toolchain builds it just as well; this is the floor, not the
  recommendation.)
- Manual constructor injection — a `container.go` with a struct and a
  constructor, not a DI framework.
- Three marker comments, which `feather init` writes and `feather doctor`
  watches over.

## Roadmap

- **v0.2** — `feather remove feature` for every kind of wiring (shipped here
  alongside v0.1 because it turned out to be tractable).
- **v0.3** — additional routers: `chi` first, then `gin`/`echo` based on demand.
- **v0.4** — template overriding: ship your own `.tmpl` files through
  `feather.yaml`.
- **v0.5** — optional AST-based injection for teams whose router and container
  have outgrown marker comments.

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md)
and the [Code of Conduct](CODE_OF_CONDUCT.md). The test suite runs the tool
against real generated projects, so `make test` is the fastest way to know your
change is safe.

## License

[MIT](LICENSE).
