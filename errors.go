package sdi

import "errors"

var (
	ErrDependencyAlreadyExists = errors.New("dependency already exists")
	ErrDependencyNotFound      = errors.New("dependency not found")
	ErrCleanupFailed           = errors.New("cleanup failed")
)
