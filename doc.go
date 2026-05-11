// Package sdi provides a small dependency injection container.
//
// The container is built around a Builder:
//
//  1. Register providers with AddProvider.
//  2. Build a root instance with BuildInstance.
//  3. Optionally render dependencies with ShowDependencies.
//
// Dependencies can be declared either as a struct (fields are treated as
// dependencies and filled by type) or as a single value type.
//
// Pointer and non-pointer types are distinct. If you need *T, register/provide
// *T explicitly.
package sdi
