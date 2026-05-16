package sdi

import "errors"

var (
	// ErrDependencyAlreadyExists is returned when a provider is registered for a type
	// that is already present in the dependency graph.
	ErrDependencyAlreadyExists = errors.New("dependency already exists")
	// ErrDependencyNotFound is returned when registering a provider whose dependencies
	// are not registered.
	ErrDependencyNotFound = errors.New("dependency not found")
	// ErrDependencyCycle indicates a dependency cycle was detected during traversal.
	ErrDependencyCycle = errors.New("dependency cycle detected")
	// ErrCleanupFailed wraps provider cleanup errors.
	ErrCleanupFailed = errors.New("cleanup failed")
	// ErrOutputWriteFailed wraps errors returned by dependency tree writers.
	ErrOutputWriteFailed = errors.New("output write failed")
	// ErrInvalidProvider is returned when a registered provider does not satisfy
	// expected reflective contract.
	ErrInvalidProvider = errors.New("invalid provider")
	// ErrBuilderNotInitialized is returned when builder is nil or its graph is nil.
	ErrBuilderNotInitialized = errors.New("builder is not initialized")
	// ErrDependencyBuildFailed is returned when building an instance fails.
	ErrDependencyBuildFailed = errors.New("dependency build failed")
	// ErrUnknownInstanceType is returned when requested instance type is not registered.
	ErrUnknownInstanceType = errors.New("unknown instance type")
	// ErrInvalidDependencyInput is returned when dependency input type is invalid.
	ErrInvalidDependencyInput = errors.New("invalid dependency input")
	// ErrInvalidDependencyValue is returned when a dependency value is missing or
	// cannot be assigned/converted to the requested type.
	ErrInvalidDependencyValue = errors.New("invalid dependency value")
)
