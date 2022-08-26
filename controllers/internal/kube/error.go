package kube

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// IsNotFound returns true if the err reason is "not found".
func IsNotFound(err error) bool {
	return apierrors.IsNotFound(err)
}

// IgnoreNotFound returns nil if err reason is "not found".
func IgnoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// IgnoreAlreadyExists returns nil if err reason is "already exists".
func IgnoreAlreadyExists(err error) error {
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
