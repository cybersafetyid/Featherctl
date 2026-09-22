# Configuration

`feather.yaml` marks the root of a project and tells featherctl where it is
allowed to generate and wire code. It is created by `feather init` and read by
every other command.

```yaml
version: 1
module: github.com/you/demo
features_dir: internal/features

router:
  path: internal/platform/router/router.go
  marker: "// feather:register-routes (do not remove this comment)"

container:
  path: internal/platform/container/container.go
  fields_marker: "// feather:container-fields (do not remove this comment)"
  wiring_marker: "// feather:container-wiring (do not remove this comment)"
```

Every value above is also the default. Deleting a line falls back to the
default; there is no way to disable a setting by blanking it.

## Top level

| Key | Default | Meaning |
| --- | --- | --- |
| `version` | `1` | Configuration schema version. A document from a newer major version is rejected rather than half-understood. |
| `module` | — (**required**) | The Go import path of the project. Generated code uses it to build import paths such as `<module>/internal/shared/httpx`. |
| `features_dir` | `internal/features` | Directory that holds one package per feature. |

`module` has no default on purpose. featherctl never invents an import path.

`features_dir`, `router.path` and `container.path` are always relative to the
project root, and must not escape it: `../elsewhere` and `/etc/passwd` are both
rejected.

## `router`

| Key | Default | Meaning |
| --- | --- | --- |
| `path` | `internal/platform/router/router.go` | File that registers feature routes. |
| `marker` | `// feather:register-routes (do not remove this comment)` | The comment featherctl inserts route registrations at. |

## `container`

Two markers are needed here because two different things get injected: a field
in the struct, and a construction block in the constructor.

| Key | Default | Meaning |
| --- | --- | --- |
| `path` | `internal/platform/container/container.go` | File that wires feature dependencies. |
| `fields_marker` | `// feather:container-fields (do not remove this comment)` | Comment inside the `Container` struct that feature dependency fields are inserted above. |
| `wiring_marker` | `// feather:container-wiring (do not remove this comment)` | Comment inside the container constructor that feature construction is inserted above. |

## Marker rules

A marker must be a Go line comment: it has to start with `//`. Beyond that it
can say anything, but it is matched **exactly** (ignoring surrounding
whitespace) against the line in the file, so changing it in `feather.yaml`
without changing the file breaks wiring — which `feather doctor` will tell you.

Generated code is inserted **immediately above** the marker line:

```go
	mux := http.NewServeMux()

	c.Order.RegisterRoutes(mux)

	// feather:register-routes (do not remove this comment)

	return mux
```

The marker itself never moves, so the injections accumulate in the order the
features were generated and stay easy to read.

Two edge cases worth knowing:

- If the marker is the only thing in a block or a struct, featherctl inserts
  just inside the braces instead.
- If a marker is missing, duplicated, or sits somewhere featherctl cannot splice
  code into safely, the command aborts and explains why. It never guesses.

## Moving the markers

All three markers may be relocated, as long as you update `feather.yaml` to
match. For example, to keep everything in one file:

```yaml
router:
  path: internal/platform/router/router.go
  marker: "// FEATHER ROUTES"

container:
  path: internal/platform/router/router.go
  fields_marker: "// FEATHER FIELDS"
  wiring_marker: "// FEATHER WIRING"
```

Two things to be careful about:

1. **The marker must be reachable where featherctl needs it.** The fields marker
   belongs inside a struct field list, the wiring marker inside a function body,
   the router marker inside the function that builds the router.
2. **`feather remove feature` has to be able to reverse what was injected.** It
   recomputes the exact lines from the marker's configuration, so if you edit
   the marker layout by hand *after* generating features, removal will refuse to
   touch the file and tell you to clean it up manually. That is intentional:
   deleting lines it is not certain about is how code gets corrupted.

## Validating a configuration

```bash
feather doctor
```

It checks that `feather.yaml` parses and validates, that every configured marker
still exists exactly once in the file it points at, that the features directory
is present, and that the project compiles. Warnings (a missing marker) do not
fail the command; a broken config or a failing build does.
