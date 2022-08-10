package kube

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReconcileError(t *testing.T) {
	err := errors.New("boom")

	terr := TransientError(err)
	require.True(t, terr.IsTransient())
	require.ErrorIs(t, terr, err)
	require.EqualError(t, terr, "transient error: boom")

	rerr := UnrecoverableError(err)
	require.False(t, rerr.IsTransient())
	require.ErrorIs(t, rerr, err)
	require.EqualError(t, rerr, "unrecoverable error: boom")
}
