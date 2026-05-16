// Package sdi provides a small dependency injection container.
//
// It is a simple tool for controlling dependencies in your application without
// taking over application lifecycle, startup flow, or runtime control. It helps
// you control how dependencies are wired while still letting you choose which
// final instance to build, when to build it, and how that instance is started
// or used. The package stays intentionally small: you register providers, build
// the instance you need, and can optionally render the dependency tree for
// debugging or inspection.
//
// The container is built around a Builder:
//
//  1. Register providers with AddProvider in dependency order.
//  2. Build a root instance with BuildInstance.
//  3. Optionally render dependencies with ShowDependencies.
//
// Dependencies can be declared either as a struct or pointer to a struct
// (exported fields are treated as dependencies and filled by type) or as a
// single value type. Promoted exported fields from embedded structs are treated
// as dependencies too, while anonymous embedded struct fields themselves are
// skipped.
//
// AddProvider requires all dependency types to be registered before the provider
// that uses them. Registering out of order returns ErrDependencyNotFound.
//
// Pointer and non-pointer types are distinct. If you need *T, register/provide
// *T explicitly.
//
// Builder and the underlying dependency graph are not thread-safe.
package sdi
