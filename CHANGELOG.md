## 0.2.0

### Added

- Automatic interface dependency resolution when exactly one registered provider implements the requested interface.
- Ambiguity detection for interface dependencies with `ErrAmbiguousDependency` when multiple providers match.
- Support for promoted exported fields from embedded structs and embedded pointer-to-struct dependencies.

### Changed

- Clarified builder cleanup behavior and dependency registration rules in package docs and tests.

## 0.1.0

Adds cleanup lifecycle support for built instances.

### Added

- `BuildInstance[T]` now returns a cleanup function that runs provider cleanup in reverse build order.
- Cleanup is also attempted for already built dependencies when instance construction fails.
- Test coverage for cleanup order, idempotent cleanup, and joined build/cleanup errors.

### Changed

- Updated documentation and the example app to show the new `BuildInstance` cleanup flow.

## 0.1.0-alpha.1

Initial alpha release.

### Added

- Small dependency injection container focused on explicit dependency control.
- Provider registration through `AddProvider`.
- Instance construction through `BuildInstance[T]`.
- Dependency graph rendering through `ShowDependencies[T]`.
- Support for struct dependencies, single-value dependencies, and pointer-to-struct dependency inputs.
- Example application in `./example`.
- CI workflows for linting, build, tests, release packaging, and coverage reporting.
- Project documentation in `README.md` and contribution guide in `CONTRIBUTING.md`.
