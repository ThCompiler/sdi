# sdi

Simple dependency injection container for Go.

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
