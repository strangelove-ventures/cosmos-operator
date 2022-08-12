package kube

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileError is a controller-specific error.
type ReconcileError interface {
	error
	// IsTransient returns true if the error is temporary.
	IsTransient() bool
}

type reconcileError struct {
	error
	isTransient bool
}

func (e reconcileError) Error() string {
	if e.isTransient {
		return fmt.Sprintf("transient error: %v", e.error)
	}
	return fmt.Sprintf("unrecoverable error: %v", e.error)
}

func (e reconcileError) IsTransient() bool { return e.isTransient }

func (e reconcileError) Unwrap() error { return e.error }

// TransientError can be recovered or retried.
func TransientError(err error) ReconcileError {
	return reconcileError{err, true}
}

// UnrecoverableError cannot be recovered and should not be retried.
func UnrecoverableError(err error) ReconcileError {
	return reconcileError{err, false}
}

// IgnoreNotFound returns nil if err reason is "not found".
func IgnoreNotFound(err error) error {
	return client.IgnoreNotFound(err)
}

// IgnoreAlreadyExists returns nil if err reason is "already exists".
func IgnoreAlreadyExists(err error) error {
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
