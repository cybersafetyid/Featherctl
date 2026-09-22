# Getting started

## Install

```bash
go install github.com/cybersafetyid/featherctl/cmd/feather@latest
```

Check it worked:

```bash
feather --version
```

## 1. Scaffold a project

```bash
feather init demo --module github.com/you/demo
cd demo
```

`--module` is required on purpose. Directory names are poor evidence of an
import path, and feather will not invent a GitHub organisation for you. When you
run `feather init` from a terminal and leave the flag out, it asks.

You get:

```
demo/
├── cmd/api/main.go
├── internal/features/           one package per feature
├── internal/platform/router/    the HTTP router and its marker
├── internal/platform/container/ dependency wiring and its markers
├── internal/shared/httpx/       JSON response and decoding helpers
├── internal/shared/validator/   input validation helpers
├── feather.yaml
├── go.mod
├── .golangci.yml
├── Makefile
├── .gitignore
└── README.md
```

The generated project has **no third-party dependencies**. It compiles as soon
as it is written:

```bash
go build ./...
go run ./cmd/api     # listens on :8080
```

If the directory already exists and has content, `feather init` asks before
touching it — and refuses outright when it is not running interactively unless
you pass `--force`. Nothing is ever written before that check passes.

## 2. Add a feature

```bash
feather new feature order
```

Six files appear under `internal/features/order/`, and two existing files gain
one small block each: the route registration in `router.go` and the dependency
wiring in `container.go`.

```bash
go build ./...
go test ./...
```

The generated tests are real tests, not placeholders: they drive the handler
through an `http.ServeMux` and stub out the repository.

Try it:

```bash
go run ./cmd/api &
curl -X POST localhost:8080/order -d '{"name":"first"}'
curl localhost:8080/order
```

Naming is forgiving. All of these produce the same package, identifiers and
route:

```bash
feather new feature userprofile
feather new feature user-profile
feather new feature userProfile
```

Add a second feature and both stay wired, in the order you created them:

```bash
feather new feature user-profile
go build ./...
```

## 3. Look before you leap

Every generating command takes `--dry-run`, which prints exactly what a real run
would do and writes nothing:

```bash
feather new feature invoice --dry-run
```

## 4. Undo a feature

```bash
feather remove feature invoice
```

The feature package is deleted and the lines featherctl injected for it are
removed, including the now-unused import. If those lines cannot be identified
with certainty — because someone edited them — nothing is deleted and the
command tells you which file to clean up by hand. Guessing is how code gets
corrupted.

## 5. Keep the project healthy

```bash
feather doctor
```

```
✔ feather.yaml: valid, module github.com/you/demo
✔ marker router: found in internal/platform/router/router.go
✔ marker container fields: found in internal/platform/container/container.go
✔ marker container wiring: found in internal/platform/container/container.go
✔ features: internal/features exists
✔ build: go build ./... succeeded

Project looks healthy.
```

`doctor` fails (exit code 1) when the project does not compile or the config is
broken, and warns when a marker comment was deleted by hand — which would stop
`feather new feature` from being able to wire anything in. That makes it a
useful CI gate:

```yaml
- name: Check the project is still generatable
  run: feather doctor
```

Skip the compile step with `--skip-build` when CI already builds the project.

## 6. Validate generated code immediately

`--validate` runs `go build ./...` right after generating, so a broken template
or a hand-edited marker cannot pass silently:

```bash
feather init demo --module github.com/you/demo --validate
feather new feature order --validate
```

## Where to go next

- [Architecture](architecture.md) explains the layout you just generated and why
  it looks like that.
- [Configuration](configuration.md) shows how to move the router, the container
  or the marker comments.
