package volsnapshot

import (
	"context"
	"errors"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockPatcher func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error

func (fn mockPatcher) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if ctx == nil {
		panic("nil context")
	}
	if len(opts) > 0 {
		panic("unexpected opts")
	}
	return fn(ctx, obj, patch)
}

var nopPatcher = mockPatcher(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
})

type mockLister func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error

func (fn mockLister) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	return fn(ctx, list, opts...)
}

var nopLister = mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
})

func TestFullNodeControl_SignalPodDeletion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var crd cosmosalpha.ScheduledVolumeSnapshot
	crd.Namespace = "default/" // Tests for slash stripping.
	crd.Name = "my-snapshot"
	crd.Spec.FullNodeRef.Namespace = "node-ns"
	crd.Spec.FullNodeRef.Name = "my-node"
	crd.Status.Candidate = &cosmosalpha.SnapshotCandidate{
		PodName: "target-pod",
	}

	t.Run("happy path", func(t *testing.T) {
		var didPatch bool
		patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			var want cosmosv1.CosmosFullNode
			want.Name = "my-node"
			want.Namespace = "node-ns"
			want.Status.ScheduledSnapshotStatus = map[string]cosmosv1.FullNodeSnapshotStatus{
				"default.my-snapshot.v1alpha1.cosmos.strange.love": {PodCandidate: "target-pod"},
			}
			require.Equal(t, client.Object(&want), obj)

			require.Equal(t, client.Merge, patch)

			didPatch = true
			return nil
		})

		control := NewFullNodeControl(patcher, nopLister)
		err := control.SignalPodDeletion(ctx, &crd)

		require.NoError(t, err)
		require.True(t, didPatch)
	})

	t.Run("patch failed", func(t *testing.T) {
		patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			return errors.New("boom")
		})

		control := NewFullNodeControl(patcher, nopLister)
		err := control.SignalPodDeletion(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}

func TestFullNodeControl_SignalPodRestoration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var crd cosmosalpha.ScheduledVolumeSnapshot
	crd.Namespace = "default/" // Tests for slash stripping.
	crd.Name = "my-snapshot"
	crd.Spec.FullNodeRef.Namespace = "node-ns"
	crd.Spec.FullNodeRef.Name = "my-node"
	crd.Status.Candidate = &cosmosalpha.SnapshotCandidate{
		PodName: "target-pod",
	}

	t.Run("happy path", func(t *testing.T) {
		var didPatch bool
		patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			var wantObj cosmosv1.CosmosFullNode
			wantObj.Name = "my-node"
			wantObj.Namespace = "node-ns"
			require.Equal(t, client.Object(&wantObj), obj)

			wantPatch := client.RawPatch(k8stypes.JSONPatchType, []byte(`[{"op":"remove","path":"/status/scheduledSnapshotStatus/default.my-snapshot.v1alpha1.cosmos.strange.love"}]`))
			require.Equal(t, wantPatch, patch)

			didPatch = true
			return nil
		})

		control := NewFullNodeControl(patcher, nopLister)
		err := control.SignalPodRestoration(ctx, &crd)

		require.NoError(t, err)
		require.True(t, didPatch)
	})

	t.Run("patch failed", func(t *testing.T) {
		patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			return errors.New("boom")
		})

		control := NewFullNodeControl(patcher, nopLister)
		err := control.SignalPodRestoration(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}

func TestFullNodeControl_ConfirmPodRestoration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var crd cosmosalpha.ScheduledVolumeSnapshot
	crd.Spec.FullNodeRef.Namespace = "default"
	crd.Spec.FullNodeRef.Name = "cosmoshub"
	crd.Status.Candidate = &cosmosalpha.SnapshotCandidate{
		PodName: "target-pod",
	}

	t.Run("happy path", func(t *testing.T) {
		var didList bool
		lister := mockLister(func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
			list.(*corev1.PodList).Items = []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "target-pod"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}},
			}

			require.Len(t, opts, 2)
			var listOpt client.ListOptions
			for _, opt := range opts {
				opt.ApplyToList(&listOpt)
			}
			require.Equal(t, "default", listOpt.Namespace)
			require.Zero(t, listOpt.Limit)
			require.Equal(t, ".metadata.controller=cosmoshub", listOpt.FieldSelector.String())

			didList = true
			return nil
		})

		control := NewFullNodeControl(nopPatcher, lister)

		err := control.ConfirmPodRestoration(ctx, &crd)
		require.NoError(t, err)

		require.True(t, didList)
	})

	t.Run("pod not restored yet", func(t *testing.T) {
		lister := mockLister(func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
			list.(*corev1.PodList).Items = []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}},
			}
			return nil
		})

		control := NewFullNodeControl(nopPatcher, lister)
		err := control.ConfirmPodRestoration(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "pod target-pod not restored yet")
	})

	t.Run("no items", func(t *testing.T) {
		control := NewFullNodeControl(nopPatcher, nopLister)
		err := control.ConfirmPodRestoration(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "pod target-pod not restored yet")
	})

	t.Run("list error", func(t *testing.T) {
		lister := mockLister(func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
			return errors.New("boom")
		})

		control := NewFullNodeControl(nopPatcher, lister)
		err := control.ConfirmPodRestoration(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "list pods: boom")
	})
}

func TestFullNodeControl_ConfirmPodDeletion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var crd cosmosalpha.ScheduledVolumeSnapshot
	crd.Spec.FullNodeRef.Namespace = "default"
	crd.Spec.FullNodeRef.Name = "cosmoshub"
	crd.Status.Candidate = &cosmosalpha.SnapshotCandidate{
		PodName: "target-pod",
	}

	t.Run("happy path", func(t *testing.T) {
		var didList bool
		lister := mockLister(func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
			list.(*corev1.PodList).Items = []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}},
			}

			require.Len(t, opts, 2)
			var listOpt client.ListOptions
			for _, opt := range opts {
				opt.ApplyToList(&listOpt)
			}
			require.Equal(t, "default", listOpt.Namespace)
			require.Zero(t, listOpt.Limit)
			require.Equal(t, ".metadata.controller=cosmoshub", listOpt.FieldSelector.String())

			didList = true
			return nil
		})

		control := NewFullNodeControl(nopPatcher, lister)

		err := control.ConfirmPodDeletion(ctx, &crd)
		require.NoError(t, err)

		require.True(t, didList)
	})

	t.Run("happy path - no items", func(t *testing.T) {
		control := NewFullNodeControl(nopPatcher, nopLister)
		err := control.ConfirmPodDeletion(ctx, &crd)

		require.NoError(t, err)
	})

	t.Run("pod not deleted yet", func(t *testing.T) {
		lister := mockLister(func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
			list.(*corev1.PodList).Items = []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "target-pod"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}},
			}
			return nil
		})

		control := NewFullNodeControl(nopPatcher, lister)
		err := control.ConfirmPodDeletion(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "pod target-pod not deleted yet")
	})

	t.Run("list error", func(t *testing.T) {
		lister := mockLister(func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
			return errors.New("boom")
		})

		control := NewFullNodeControl(nopPatcher, lister)
		err := control.ConfirmPodDeletion(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "list pods: boom")
	})
}
