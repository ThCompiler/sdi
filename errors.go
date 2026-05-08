package sdi

import "errors"

var (
	ErrDependencyAlreadyExists = errors.New("dependency already exists")
	ErrDependencyNotFound      = errors.New("dependency not found")
	ErrDependencyCycle         = errors.New("dependency cycle detected")
	ErrCleanupFailed           = errors.New("cleanup failed")
	ErrOutputWriteFailed       = errors.New("output write failed")
	ErrInvalidProvider         = errors.New("invalid provider")
	ErrBuilderNotInitialized   = errors.New("builder is not initialized")
	ErrDependencyBuildFailed   = errors.New("dependency build failed")
	ErrUnknownInstanceType     = errors.New("unknown instance type")
	ErrInvalidDependencyInput  = errors.New("invalid dependency input")
	ErrInvalidDependencyValue  = errors.New("invalid dependency value")
)

var (
	ErrSeekOutOfRange   = errors.New("seek out of range")
	ErrUnknowSeekWhence = errors.New("unknow seek whence")
)
