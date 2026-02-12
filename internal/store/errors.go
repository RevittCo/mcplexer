package store

import "errors"

var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates the resource already exists (unique constraint).
	ErrAlreadyExists = errors.New("already exists")

	// ErrConflict indicates a concurrent modification conflict.
	ErrConflict = errors.New("conflict")
)
