package cosmos

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestStatusCollection_Synced(t *testing.T) {
	t.Parallel()

	var coll StatusCollection
	require.Empty(t, coll.Synced())
	require.Empty(t, coll.SyncedPods())

	var catchingUp CometStatus
	catchingUp.Result.SyncInfo.CatchingUp = true

	coll = StatusCollection{
		{Pod: &corev1.Pod{}, Status: catchingUp},
		{Pod: &corev1.Pod{}, Err: errors.New("some error")},
	}

	require.Empty(t, coll.Synced())
	require.Empty(t, coll.SyncedPods())

	var pod corev1.Pod
	pod.Name = "in-sync"
	coll = append(coll, StatusItem{Pod: &pod})

	require.Len(t, coll.Synced(), 1)
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
	require.Equal(t, "new", coll[0].GetPod().Name)
	require.WithinDuration(t, time.Now(), coll[0].Timestamp(), 10*time.Second)

	UpsertPod(&coll, &corev1.Pod{})
	require.Len(t, coll, 2)

	ts := time.Now()
	status := CometStatus{ID: 1}
	err := errors.New("some error")
	coll = StatusCollection{
		{Pod: &pod, TS: ts, Status: status, Err: err},
	}
	var pod2 corev1.Pod
	pod2.UID = "1"
	pod2.Name = "new2"
	UpsertPod(&coll, &pod2)

	require.Len(t, coll, 1)
	want := StatusItem{Pod: &pod2, TS: ts, Status: status, Err: err}
	require.Equal(t, want, coll[0])
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

	coll = append(coll, StatusItem{Pod: &pod})
	coll = append(coll, StatusItem{Pod: &pod2})

	IntersectPods(&coll, []corev1.Pod{pod})
	require.Len(t, coll, 1)
	require.Equal(t, "1", string(coll[0].GetPod().UID))
}
