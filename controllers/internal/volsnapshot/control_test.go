package volsnapshot

import (
	"context"
	"errors"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/fullnode"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClientReader struct {
	GotOpts []client.ListOption

	PodItems []corev1.Pod
	ListErr  error
}

func (m *mockClientReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	panic("implement me")
}

func (m *mockClientReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.GotOpts = opts
	list.(*corev1.PodList).Items = m.PodItems
	return m.ListErr
}

type mockPodFinder func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error)

func (fn mockPodFinder) SyncedPod(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
	return fn(ctx, candidates)
}

func TestVolumeSnapshotControl_FindCandidate(t *testing.T) {
	var (
		ctx            = context.Background()
		readyCondition = corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue}
	)

	var crd cosmosalpha.ScheduledVolumeSnapshot
	crd.Spec.SourceRef.Namespace = "strangelove"
	crd.Spec.SourceRef.Name = "cosmoshub"

	t.Run("happy path", func(t *testing.T) {
		pods := make([]corev1.Pod, 3)
		for i := range pods {
			pods[i].Status.Conditions = []corev1.PodCondition{readyCondition}
		}
		var mClient mockClientReader
		mClient.PodItems = pods

		control := NewVolumeSnapshotControl(&mClient, mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
			require.NotNil(t, ctx)
			require.Equal(t, ptrSlice(pods), candidates)

			// Purposefully using PodBuilder to cross-test any breaking changes in PodBuilder which affects
			// finding the PVC name.
			var fullnodeCRD cosmosv1.CosmosFullNode
			fullnodeCRD.Name = "osmosis"
			candidate := fullnode.NewPodBuilder(&fullnodeCRD).WithOrdinal(1).Build()
			return candidate, nil
		}))

		got, err := control.FindCandidate(ctx, &crd)
		require.NoError(t, err)

		require.Equal(t, "osmosis-1", got.PodName)
		require.Equal(t, "pvc-osmosis-1", got.PVCName)

		require.Len(t, mClient.GotOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "strangelove", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=cosmoshub", listOpt.FieldSelector.String())
	})

	t.Run("list error", func(t *testing.T) {
		var mClient mockClientReader
		mClient.ListErr = errors.New("no list for you")
		control := NewVolumeSnapshotControl(&mClient, mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
			panic("should not be called")
		}))

		_, err := control.FindCandidate(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "no list for you")
	})

	t.Run("synced pod error", func(t *testing.T) {
		pods := make([]corev1.Pod, 2)
		for i := range pods {
			pods[i].Status.Conditions = []corev1.PodCondition{readyCondition}
		}
		var mClient mockClientReader
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
			Pods []corev1.Pod
		}{
			{nil},
			{make([]corev1.Pod, 0)},
			{make([]corev1.Pod, 3)},      // no pods ready
			{[]corev1.Pod{readyPod, {}}}, // 1 pod ready
		} {
			var mClient mockClientReader
			mClient.PodItems = tt.Pods

			control := NewVolumeSnapshotControl(&mClient, mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
				panic("should not be called")
			}))

			_, err := control.FindCandidate(ctx, &crd)

			require.Error(t, err, tt)
			require.EqualError(t, err, "2 or more pods must be in a ready state to prevent downtime", tt)
		}
	})
}
