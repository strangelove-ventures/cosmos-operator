package kube

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReconcileError(t *testing.T) {
	t.Parallel()

	err := errors.New("boom")

	terr := TransientError(err)
	require.True(t, terr.IsTransient())
	require.ErrorIs(t, terr, err)
	require.EqualError(t, terr, "boom")

	rerr := UnrecoverableError(err)
	require.False(t, rerr.IsTransient())
	require.ErrorIs(t, rerr, err)
	require.EqualError(t, rerr, "boom")
}

func TestReconcileErrors(t *testing.T) {
	t.Parallel()

	t.Run("transient", func(t *testing.T) {
		errs := &ReconcileErrors{}
		require.False(t, errs.Any())

		errs.Append(TransientError(errors.New("boom1")))
		errs.Append(TransientError(errors.New("boom2")))

		require.True(t, errs.Any())

		require.EqualError(t, errs, "boom1; boom2")
		require.True(t, errs.IsTransient())
	})

	t.Run("unrecoverable", func(t *testing.T) {
		errs := &ReconcileErrors{}
		errs.Append(TransientError(errors.New("boom1")))
		errs.Append(UnrecoverableError(errors.New("boom2")))
		errs.Append(TransientError(errors.New("boom3")))

		require.EqualError(t, errs, "boom1; boom2; boom3")
		require.False(t, errs.IsTransient())
	})
}

func TestErrors(t *testing.T) {
	t.Run("single error", func(t *testing.T) {
		err := TransientError(errors.New("boom"))
		got := Errors(err)

		require.Len(t, got, 1)
		require.Equal(t, err, got[0])
	})

	t.Run("multiple errors", func(t *testing.T) {
		errs := &ReconcileErrors{}
		errs.Append(TransientError(errors.New("1")))
		errs.Append(UnrecoverableError(errors.New("2")))
		got := Errors(errs)

		require.Len(t, got, 2)
		require.EqualError(t, got[0], "1")
		require.EqualError(t, got[1], "2")
	})
}
