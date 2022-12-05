package fullnode

import (
	"context"
	"testing"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Using a custom mock because VolumeSnapshotControl manages multiple resources: pods and VolumeSnapshots
type volumeSnapshotClient struct {
	Pods        []corev1.Pod
	ListErr     error
	GotListOpts []client.ListOption
}

func (v *volumeSnapshotClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	panic("implement me")
}

func (v *volumeSnapshotClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	v.GotListOpts = opts
	list.(*corev1.PodList).Items = v.Pods
	return v.ListErr
}

func (v *volumeSnapshotClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	panic("implement me")
}

func (v *volumeSnapshotClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	panic("implement me")
}

func (v *volumeSnapshotClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	panic("implement me")
}

func (v *volumeSnapshotClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	panic("implement me")
}

func (v *volumeSnapshotClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

func (v *volumeSnapshotClient) Scheme() *runtime.Scheme {
	panic("implement me")
}

type mockPodFinder func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error)

func (fn mockPodFinder) SyncedPod(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
	return fn(ctx, candidates)
}

func TestVolumeSnapshotControl_FindCandidate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path - first snapshot", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "cosmoshub"
		crd.Namespace = "test"
		crd.Spec.VolumeSnapshot = &cosmosv1.VolumeSnapshotSpec{
			Schedule: "0 6 * * *", // Every day at 6:00
		}
		fiveAM := time.Date(2022, time.December, 1, 5, 0, 0, 0, time.UTC)
		crd.Status.VolumeSnapshot = &cosmosv1.VolumeSnapshotStatus{ActivatedAt: metav1.NewTime(fiveAM)}

		var mClient volumeSnapshotClient
		mClient.Pods = make([]corev1.Pod, 3)

		mockFinder := mockPodFinder(func(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
			require.NotNil(t, ctx)
			require.Len(t, candidates, 3)
			var syncedPod corev1.Pod
			syncedPod.Name = "found-me"
			return &syncedPod, nil
		})
		control := NewVolumeSnapshotControl(&mClient, mockFinder)
		sixAM := time.Date(2022, time.December, 1, 6, 0, 0, 0, time.UTC)
		got, err := control.FindCandidate(ctx, &crd, sixAM)

		require.NoError(t, err)

		require.True(t, got.Valid())
		require.Equal(t, "found-me", got.PodName())
		require.Equal(t, "pvc-found-me", got.PVCName())
		require.False(t, got.NeedsRequeue())

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=cosmoshub", listOpt.FieldSelector.String())
	})

	t.Run("happy path - completed snapshot", func(t *testing.T) {
		t.Fatal("TODO")
	})

	t.Run("volume snapshots not configured", func(t *testing.T) {
		control := NewVolumeSnapshotControl(nil, nil)

		var crd cosmosv1.CosmosFullNode
		got, err := control.FindCandidate(ctx, &crd, time.Now())

		require.NoError(t, err)
		require.False(t, got.Valid())
	})

	t.Run("invalid cron", func(t *testing.T) {
		t.Fatal("TODO")
	})

	t.Run("not time for scheduled run", func(t *testing.T) {

	})

	t.Run("volume snapshot already running", func(t *testing.T) {
		t.Fatal("TODO")
	})
}
