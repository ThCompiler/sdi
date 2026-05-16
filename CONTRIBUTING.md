# Contributing

Thanks for contributing to `sdi`.

## Development Setup

Requirements:

- Go `>= 1.25.7`
- `make`

Install local tools:

```bash
make install
make install-lint
```

## Workflow

1. Create a branch for your change.
2. Make the smallest correct change.
3. Add or update tests for behavior changes.
4. Run the local checks before opening a pull request.

Recommended local checks:

```bash
make lint
make test
make test-coverage
```

## Code Guidelines

- Keep changes focused and minimal.
- Preserve the package style and public API unless the change intentionally updates it.
- Register dependencies in tests in dependency order, just like production code.
- Do not introduce concurrent access to a shared `Builder` or dependency graph without external synchronization.

## Tests

- Add tests for new behavior and regressions.
- Prefer descriptive test names.
- Use `github.com/stretchr/testify/require` for assertions.
- Use `t.Parallel()` when tests are independent.

## Pull Requests

- Explain what changed and why.
- Mention any API or behavior changes explicitly.
- Keep pull requests scoped to one concern when possible.
- Make sure CI is green before requesting review.

## License

By contributing, you agree that your contributions are provided under the project's
Apache License 2.0.
