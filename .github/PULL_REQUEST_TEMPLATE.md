## What this changes

<!-- One or two sentences. Focus on the why, not the what. -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Documentation
- [ ] Refactor or internal cleanup
- [ ] CI, tooling or release plumbing

## Checklist

- [ ] `make test` passes locally (this includes the end-to-end suite)
- [ ] `make lint` passes
- [ ] New and changed behaviour is covered by tests
- [ ] Exported identifiers have doc comments
- [ ] Golden fixtures were regenerated with `make golden` if templates changed,
      and I reviewed the diff under `internal/generator/testdata/`
- [ ] `CHANGELOG.md` has an entry under `[Unreleased]` for user-visible changes
- [ ] Commits follow Conventional Commits (`feat:`, `fix:`, `docs:`, ...)

## How it was verified

<!-- Which command did you run, and what did you see? If this touches
     generation, paste the generated code or the `feather doctor` output. -->

## Related issues

<!-- Closes #123 -->
