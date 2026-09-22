# Architecture

This document explains the layout `feather` generates, why it is shaped that
way, and how featherctl itself is built.

## Vertical slice architecture, in plain terms

A layered project groups code by technical role: every handler in `handlers/`,
every service in `services/`, every repository in `repositories/`. Adding one
feature means editing four directories, and the feature is never in one place.

A vertical slice groups code by capability instead. Everything one feature needs
— the HTTP handler, the business rules, the storage and the tests — lives in one
package:

```
internal/features/order/
├── model.go
├── repository.go
├── service.go
├── handler.go
├── handler_test.go
└── service_test.go
```

The payoffs are concrete:

- **Reading a feature is one directory.** You never have to hold four layers in
  your head to answer "how does creating an order work?".
- **Tests live next to what they test** and share its package, so they can reach
  internals when that is genuinely the right thing to do.
- **Boundaries stay honest.** The slice depends on *interfaces* it declares
  itself (`Repository` in `repository.go`), not on something in a shared
  `repositories/` folder.
- **Deleting is safe.** Remove the package, remove the two wiring lines.

The cost is some duplication between slices. That is a feature, not a bug: a
slice you can read end to end is worth more than a shared abstraction that
guesses wrong.

## The three seams

Only three things are shared, and each one is deliberate:

```
internal/platform/router/router.go      the HTTP router
internal/platform/container/container.go dependency wiring
internal/shared/                          helpers with no feature knowledge
```

`router.New(c *container.Container)` builds an `http.ServeMux`, mounts every
wired feature on it and returns it. It uses the standard library's method-aware
router (`GET /order`, `POST /order`), so a generated project pulls in nothing
beyond the standard library.

`container.New()` constructs shared dependencies and then every feature, using
plain constructor injection — a struct, a constructor and a handful of
assignments. There is no DI framework, and there is nothing to regenerate: the
container is ordinary Go that a reader can follow top to bottom.

`internal/shared/` holds the small things that are genuinely universal — writing
a JSON response, decoding a request body with a size limit, accumulating field
validation errors. Nothing in `shared/` imports a feature.

Dependency direction is one-way:

```
cmd/api → internal/platform/{router,container} → internal/features/* → internal/shared/*
```

Nothing imports `cmd/`, and no feature imports another feature.

## How featherctl wires a feature in

This is the part that makes the tool worth using, so it is worth understanding
what it actually does.

featherctl does **not** rewrite your files. It looks for a marker comment,
inserts code at it, and then formats the result:

```go
func New(c *container.Container) *http.ServeMux {
	mux := http.NewServeMux()

	c.Order.RegisterRoutes(mux)

	// feather:register-routes (do not remove this comment)

	return mux
}
```

The implementation uses [`dave/dst`](https://github.com/dave/dst), a fork of
`go/ast` that preserves decorations — comments, blank lines, the grouping of
imports — through a parse/print round trip. Concretely, that means:

1. The file is parsed into a *decorated* syntax tree.
2. The marker comment is located. It is usually attached to the next node, but
   dst also attaches comments that sit at the head or tail of a block or struct
   to the list itself; both shapes are handled.
3. The new statements or fields are spliced in **above** the marker.
4. Missing imports are added, then the whole file is run through goimports (as a
   library), which also removes imports that just became unused.

Everything outside the insertion point is reproduced byte for byte. That is the
property that makes it safe to point at a file a human has been editing.

The design rules that follow from it:

- **No marker, no injection.** A missing marker aborts with an error naming the
  file and the marker. featherctl never picks a plausible-looking location.
- **Two markers is one too many.** A duplicated marker aborts, because choosing
  between them would be a guess.
- **Deterministic removal.** `feather remove feature` recomputes the exact lines
  it would have injected and deletes them one by one. A line that is missing or
  appears twice aborts the whole removal, and the message says which file to fix
  by hand.
- **Unsupported positions abort.** If a marker ended up somewhere featherctl
  cannot splice code into safely, the run fails rather than doing something
  clever.

## How it relates to other Go scaffolding tools

Tools like [`go-blueprint`](https://github.com/melkeydev/go-blueprint),
`gonew`, `gohex` and `structify` solve a different problem: they generate a
*project*, once, at the start, usually with a chosen framework and database
stack.

featherctl **complements** them rather than replacing them:

| | Whole-project scaffolds | featherctl |
| --- | --- | --- |
| When it runs | Once, at the start | Every time you add a feature |
| What it knows | Frameworks, databases, HTTP stacks | Your project's own router and container |
| Output | A project you then edit by hand | One slice, already wired into what exists |
| Framework lock-in | Usually yes | None: `net/http` and manual wiring |

If you already generated a project with one of those tools, featherctl will not
adopt it — it expects the markers it writes itself. If you want a project plus a
recurring way to add features to it, that is exactly the gap this fills.

## How featherctl itself is built

```
cmd/feather/            the binary; parses nothing, decides nothing
internal/cli/           cobra commands: flags in, report out
internal/generator/     orchestration: templates, wiring, reports, doctor
internal/wiring/        dst-based marker injection and removal
internal/config/        feather.yaml schema, defaults, validation
internal/fsutil/        safe writes and conflict detection
templates/              embedded with embed.FS, so the binary needs no files
e2e/                    builds a real project and runs `go build ./...` on it
```

The rules the code follows:

- `internal/cli` is thin. Commands parse flags, call the generator and render
  the report. No business logic lives in a `RunE`.
- `internal/generator`, `wiring`, `config` and `fsutil` are independently
  testable and never write to standard output or call `os.Exit`.
- Nothing imports `cmd/`.
- Generated code is not described in prose and then written by hand: it comes
  from templates under `templates/`, and every template is covered by a golden
  file test under `internal/generator/testdata/`.

The end-to-end suite is the one that matters most: it builds the CLI, uses it to
scaffold a project, generates two features, removes one, and asserts that
`go build ./...` and `go test ./...` succeed at every step. If that test passes,
the core promise holds.
