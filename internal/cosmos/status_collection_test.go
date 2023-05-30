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

func TestUpsertPod(t *testing.T) {
	t.Parallel()

	var coll StatusCollection
	var pod corev1.Pod
	pod.UID = "1"
	UpsertPod(&coll, &pod)

	require.Len(t, coll, 1)

	pod.Name = "new"
	UpsertPod(&coll, &pod)

	require.Len(t, coll, 1)
	require.Equal(t, "new", coll[0].Pod().Name)

	UpsertPod(&coll, &corev1.Pod{})
	require.Len(t, coll, 2)
}

func TestIntersectPods(t *testing.T) {
	t.Parallel()

	var coll StatusCollection
	var pod corev1.Pod
	pod.UID = "1"

	IntersectPods(&coll, []corev1.Pod{pod})
	require.NotNil(t, coll)
	require.Len(t, coll, 0)

	var pod2 corev1.Pod
	pod2.UID = "2"

	coll = append(coll, StatusItem{pod: &pod})
	coll = append(coll, StatusItem{pod: &pod2})

	IntersectPods(&coll, []corev1.Pod{pod})
	require.Len(t, coll, 1)
	require.Equal(t, "1", string(coll[0].Pod().UID))
}
