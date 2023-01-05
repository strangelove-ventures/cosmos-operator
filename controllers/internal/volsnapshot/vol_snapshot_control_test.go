package volsnapshot

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClient struct {
	GotListOpts []client.ListOption
	PodItems    []corev1.Pod
	ListErr     error

	GotCreateObj client.Object
	CreateErr    error
}

func (m *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if ctx == nil {
		panic("nil context")
	}
	if len(opts) > 0 {
		panic(fmt.Errorf("expected 0 opts, got %d", len(opts)))
	}
	m.GotCreateObj = obj
	return m.CreateErr
}

func (m *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.GotListOpts = opts
	list.(*corev1.PodList).Items = m.PodItems
	return m.ListErr
}

type mockPodFinder func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error)

func (fn mockPodFinder) SyncedPod(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
	return fn(ctx, candidates)
}

var panicFinder = mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
	panic("should not be called")
})

func TestVolumeSnapshotControl_FindCandidate(t *testing.T) {
	t.Parallel()

	var (
		ctx            = context.Background()
		readyCondition = corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue}
	)

	var crd cosmosalpha.ScheduledVolumeSnapshot
	crd.Spec.FullNodeRef.Namespace = "strangelove"
	crd.Spec.FullNodeRef.Name = "cosmoshub"

	t.Run("happy path", func(t *testing.T) {
		pods := make([]corev1.Pod, 3)
		for i := range pods {
			pods[i].Status.Conditions = []corev1.PodCondition{readyCondition}
		}
		var mClient mockClient
		mClient.PodItems = pods

		var fullnodeCRD cosmosv1.CosmosFullNode
		fullnodeCRD.Name = "osmosis"
		// Purposefully using PodBuilder to cross-test any breaking changes in PodBuilder which affects
		// finding the PVC name.
		candidate := fullnode.NewPodBuilder(&fullnodeCRD).WithOrdinal(1).Build()

		control := NewVolumeSnapshotControl(&mClient, mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
			require.NotNil(t, ctx)
			require.Equal(t, ptrSlice(pods), candidates)

			return candidate, nil
		}))

		got, err := control.FindCandidate(ctx, &crd)
		require.NoError(t, err)

		require.Equal(t, "osmosis-1", got.PodName)
		require.Equal(t, "pvc-osmosis-1", got.PVCName)
		require.NotEmpty(t, got.PodLabels)
		require.Equal(t, candidate.Labels, got.PodLabels)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "strangelove", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=cosmoshub", listOpt.FieldSelector.String())
	})

	t.Run("custom min available", func(t *testing.T) {
		var pod corev1.Pod
		pod.Name = "found-me"
		pod.Status.Conditions = []corev1.PodCondition{readyCondition}
		var mClient mockClient
		mClient.PodItems = []corev1.Pod{pod}

		control := NewVolumeSnapshotControl(&mClient, mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
			return &pod, nil
		}))

		availCRD := crd.DeepCopy()
		availCRD.Spec.MinAvailable = 1

		got, err := control.FindCandidate(ctx, availCRD)

		require.NoError(t, err)
		require.Equal(t, "found-me", got.PodName)
	})

	t.Run("list error", func(t *testing.T) {
		var mClient mockClient
		mClient.ListErr = errors.New("no list for you")
		control := NewVolumeSnapshotControl(&mClient, panicFinder)

		_, err := control.FindCandidate(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "no list for you")
	})

	t.Run("synced pod error", func(t *testing.T) {
		pods := make([]corev1.Pod, 2)
		for i := range pods {
			pods[i].Status.Conditions = []corev1.PodCondition{readyCondition}
		}
		var mClient mockClient
		mClient.PodItems = pods

		control := NewVolumeSnapshotControl(&mClient, mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
			return nil, errors.New("pod sync error")
		}))

		_, err := control.FindCandidate(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "pod sync error")
	})

	t.Run("not enough ready pods", func(t *testing.T) {
		var readyPod corev1.Pod
		readyPod.Status.Conditions = []corev1.PodCondition{readyCondition}

		for _, tt := range []struct {
			Pods    []corev1.Pod
			WantErr string
		}{
			{nil, "list operation returned no pods"},
			{make([]corev1.Pod, 0), "list operation returned no pods"},
			{make([]corev1.Pod, 3), "2 or more pods must be ready to prevent downtime, found 0 ready"},      // no pods ready
			{[]corev1.Pod{readyPod, {}}, "2 or more pods must be ready to prevent downtime, found 1 ready"}, // 1 pod ready
		} {
			var mClient mockClient
			mClient.PodItems = tt.Pods

			control := NewVolumeSnapshotControl(&mClient, panicFinder)

			_, err := control.FindCandidate(ctx, &crd)

			require.Error(t, err, tt)
			require.EqualError(t, err, tt.WantErr, tt)
		}
	})
}

func TestVolumeSnapshotControl_CreateSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		var mClient mockClient
		control := NewVolumeSnapshotControl(&mClient, panicFinder)
		// Use time.Local to ensure we format with UTC.
		now := time.Date(2022, time.September, 1, 2, 3, 0, 0, time.UTC)
		control.now = func() time.Time {
			return now
		}

		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Name = "my-snapshot"
		crd.Namespace = "strangelove"
		crd.Spec.VolumeSnapshotClassName = "my-snap-class"

		labels := map[string]string{
			"test":               "labels",
			kube.ControllerLabel: "should not see me",
			kube.ComponentLabel:  "should not see me",
		}
		candidate := Candidate{
			PodLabels: labels,
			PodName:   "chain-1",
			PVCName:   "pvc-chain-1",
		}
		err := control.CreateSnapshot(ctx, &crd, candidate)

		require.NoError(t, err)
		require.NotNil(t, mClient.GotCreateObj)

		got := mClient.GotCreateObj.(*snapshotv1.VolumeSnapshot)
		require.Equal(t, "strangelove", got.Namespace)
		const wantName = "my-snapshot-202209010203"
		require.Equal(t, wantName, got.Name)

		require.Equal(t, "my-snap-class", *got.Spec.VolumeSnapshotClassName)
		require.Equal(t, "pvc-chain-1", *got.Spec.Source.PersistentVolumeClaimName)
		require.Nil(t, got.Spec.Source.VolumeSnapshotContentName)

		wantLabels := map[string]string{
			"test":               "labels",
			kube.ControllerLabel: "cosmos-operator",
			kube.ComponentLabel:  "ScheduledVolumeSnapshot",
		}
		require.Equal(t, wantLabels, got.Labels)

		wantStatus := &cosmosalpha.VolumeSnapshotStatus{
			Name:      wantName,
			StartedAt: metav1.NewTime(now),
		}
		require.Equal(t, wantStatus, crd.Status.LastSnapshot)
	})

	t.Run("nil pod labels", func(t *testing.T) {
		var mClient mockClient
		control := NewVolumeSnapshotControl(&mClient, panicFinder)
		var crd cosmosalpha.ScheduledVolumeSnapshot

		err := control.CreateSnapshot(ctx, &crd, Candidate{})

		require.NoError(t, err)
		require.NotNil(t, mClient.GotCreateObj)

		got := mClient.GotCreateObj.(*snapshotv1.VolumeSnapshot)

		wantLabels := map[string]string{
			kube.ControllerLabel: "cosmos-operator",
			kube.ComponentLabel:  "ScheduledVolumeSnapshot",
		}
		require.Equal(t, wantLabels, got.Labels)
	})

	t.Run("create error", func(t *testing.T) {
		var mClient mockClient
		mClient.CreateErr = errors.New("boom")
		control := NewVolumeSnapshotControl(&mClient, panicFinder)
		var crd cosmosalpha.ScheduledVolumeSnapshot
		err := control.CreateSnapshot(ctx, &crd, Candidate{})

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}
