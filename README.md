# sdi

Simple dependency injection container for Go.

## Usage

High level flow:

1. Create a builder: `b := sdi.NewBuilder()`
2. Register providers with `sdi.AddProvider`.
3. Build a root instance with `sdi.BuildInstance[T]`.
4. (Optional) Print the dependency tree with `sdi.ShowDependencies[T]`.

### Providers and dependencies

To register a provider:

```go
sdi.AddProvider[InstanceType, DependenciesType](b, provider)
```

`DependenciesType` controls what is passed to `Provider.GetInstance(ctx, deps)`:

- If it is a `struct`, its exported fields are treated as dependencies and are filled by type.
- Otherwise it is treated as a single dependency value.

Pointer and non-pointer types are distinct. If you need `*T`, register/provide `*T` explicitly.

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
make fmt
make lint
make test
```

## Release

Create and push a tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions will run CI for pushes and pull requests, then build release archives for tagged versions.
