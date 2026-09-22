# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `feather init <project-name>` scaffolds a vertical-slice Go project: router,
  dependency container, shared HTTP and validation helpers, `feather.yaml`,
  `golangci-lint` configuration, `Makefile`, `README.md` and tests-ready feature
  directory. Supports `--module`, `--force`, `--dry-run` and `--validate`.
- `feather new feature <name>` generates a complete feature slice — model,
  repository, service, handler and two table-driven test files — and wires it
  into the router and the dependency container with no manual edits. Supports
  `--force`, `--dry-run` and `--validate`.
- `feather remove feature <name>` deletes a feature and reverses its wiring,
  refusing to guess when the injected lines have been edited by hand.
- `feather doctor [path]` validates `feather.yaml`, checks that every injection
  marker still exists exactly once, and compiles the project. Suitable as a CI
  gate.
- Marker-comment auto-wiring built on `dave/dst`, which preserves the surrounding
  comments and formatting byte for byte.
- Feature names may be written as `order`, `user-profile` or `userProfile`.
- Golden-file tests for every template, unit tests for wiring, config and
  filesystem helpers, and an end-to-end test that scaffolds a real project and
  runs `go build ./...` and `go test ./...` against it.
- GitHub Actions workflows for CI (lint, test, build on Linux, macOS and
  Windows) and releases via GoReleaser.

[Unreleased]: https://github.com/cybersafetyid/featherctl/commits/main
