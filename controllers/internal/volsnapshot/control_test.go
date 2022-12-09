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

		require.Len(t, mClient.GotOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotOpts {
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
		var mClient mockClientReader
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
			Pods    []corev1.Pod
			WantErr string
		}{
			{nil, "list operation returned no pods"},
			{make([]corev1.Pod, 0), "list operation returned no pods"},
			{make([]corev1.Pod, 3), "2 or more pods must be ready to prevent downtime, found 0 ready"},      // no pods ready
			{[]corev1.Pod{readyPod, {}}, "2 or more pods must be ready to prevent downtime, found 1 ready"}, // 1 pod ready
		} {
			var mClient mockClientReader
			mClient.PodItems = tt.Pods

			control := NewVolumeSnapshotControl(&mClient, mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
				panic("should not be called")
			}))

			_, err := control.FindCandidate(ctx, &crd)

			require.Error(t, err, tt)
			require.EqualError(t, err, tt.WantErr, tt)
		}
	})
}
