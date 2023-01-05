package volsnapshot

import (
	"context"
	"errors"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
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

		control := NewFullNodeControl(patcher)
		err := control.SignalPodDeletion(ctx, &crd)

		require.NoError(t, err)
		require.True(t, didPatch)
	})

	t.Run("patch failed", func(t *testing.T) {
		patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			return errors.New("boom")
		})

		control := NewFullNodeControl(patcher)
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

		control := NewFullNodeControl(patcher)
		err := control.SignalPodRestoration(ctx, &crd)

		require.NoError(t, err)
		require.True(t, didPatch)
	})

	t.Run("patch failed", func(t *testing.T) {
		patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			return errors.New("boom")
		})

		control := NewFullNodeControl(patcher)
		err := control.SignalPodRestoration(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}
