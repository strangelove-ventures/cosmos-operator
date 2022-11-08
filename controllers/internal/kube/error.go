package kube

import (
	"strings"

	"github.com/samber/lo"
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

// IsAlreadyExists determines if the error indicates that a specified resource already exists.
// It supports wrapped errors and returns false when the error is nil.
func IsAlreadyExists(err error) bool {
	return apierrors.IsAlreadyExists(err)
}

// ReconcileErrors is a collection of ReconcileError
type ReconcileErrors struct {
	errs []ReconcileError
}

func (errs *ReconcileErrors) Error() string {
	all := lo.Map(errs.errs, func(err ReconcileError, i int) string { return err.Error() })
	return strings.Join(all, "; ")
}

// IsTransient returns true if all errors are transient. False if at least one is not transient.
func (errs *ReconcileErrors) IsTransient() bool {
	for _, err := range errs.errs {
		if !err.IsTransient() {
			return false
		}
	}
	return true
}

// Append adds the ReconcileError.
func (errs *ReconcileErrors) Append(err ReconcileError) {
	errs.errs = append(errs.errs, err)
}

// Any returns true if any errors were collected.
func (errs *ReconcileErrors) Any() bool {
	return len(errs.errs) > 0
}
