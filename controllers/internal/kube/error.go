package fullnode

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
