package cosmos

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestStatusCollection_SyncedPods(t *testing.T) {
	t.Parallel()

	var coll StatusCollection
	require.Empty(t, coll.SyncedPods())

	var catchingUp CometStatus
	catchingUp.Result.SyncInfo.CatchingUp = true

	coll = StatusCollection{
		{pod: &corev1.Pod{}, status: catchingUp},
		{pod: &corev1.Pod{}, err: errors.New("some error")},
	}

	require.Empty(t, coll.SyncedPods())

	var pod corev1.Pod
	pod.Name = "in-sync"
	coll = append(coll, StatusItem{pod: &pod})

	require.Len(t, coll.SyncedPods(), 1)
	require.Equal(t, "in-sync", coll.SyncedPods()[0].Name)
}
