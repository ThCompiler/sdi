# sdi

[![CI](https://github.com/ThCompiler/sdi/actions/workflows/ci.yml/badge.svg)](https://github.com/ThCompiler/sdi/actions/workflows/ci.yml)
[![Release](https://github.com/ThCompiler/sdi/actions/workflows/release.yml/badge.svg)](https://github.com/ThCompiler/sdi/actions/workflows/release.yml)

Simple dependency injection container for Go.

## Overview

`sdi` is a small dependency injection container and a simple tool for controlling
dependencies in your application.
It does not take over application lifecycle, startup flow, or runtime control.
Instead, it helps you control how dependencies are wired while still letting you
decide which final instance you want to build, when you want to build it, and
how that instance will be started or used.

## Usage

High level flow:

1. Create a builder: `b := sdi.NewBuilder()`
2. Register providers with `sdi.AddProvider` in dependency order.
3. Build a root instance with `sdi.BuildInstance[T]` and call the returned cleanup function when you're done.
4. (Optional) Print the dependency tree with `sdi.ShowDependencies[T]`.

Cleanup uses an internal timeout-bound context configured on the builder. To override the default:

```go
b := sdi.NewBuilder(sdi.WithCleanupTimeout(10 * time.Second))
```

The returned cleanup function uses this internal timeout and does not accept an external context.

### Providers and dependencies

To register a provider:

```go
sdi.AddProvider[InstanceType, DependenciesType](b, provider)
```

`DependenciesType` controls what is passed to `Provider.GetInstance(ctx, deps)`:

- If it is a `struct` or pointer to a `struct`, its exported fields are treated as dependencies and are filled by type.
- Promoted exported fields from embedded structs are treated as dependencies too.
- For embedded pointer-to-struct fields: SDI will initialize nil embedded pointers only if the embedded field is settable via reflection (i.e. the embedded type/field is exported). Promoted fields behind an unexported embedded pointer are not supported.
- Anonymous embedded struct fields themselves are skipped.
- Any other type is treated as a single dependency value.

Providers must be registered in dependency order. If provider `A` depends on `B`,
register `B` first and `A` second. `AddProvider` only links to dependencies that are
already present in the builder, so registering out of order returns
`ErrDependencyNotFound`.

Pointer and non-pointer types are distinct. If you need `*T`, register/provide `*T` explicitly.

`Builder` and the underlying dependency graph are not thread-safe. Do not call
`AddProvider`, `BuildInstance`, or `ShowDependencies` concurrently on the same
builder without external synchronization.

### Example

This repository contains a runnable example in `./example`.

```bash
go run ./example
```

The example demonstrates:

- Providing a `Config`.
- Providing a `Logger`.
- Building a `Service` that depends on both (via a struct deps type).
- Printing the dependency tree.

## Development

```bash
make lint
make test
make test-coverage
```

See `CONTRIBUTING.md` for contribution and local development guidelines.

GitHub Actions run lint, build, tests, coverage reporting, and release packaging.
