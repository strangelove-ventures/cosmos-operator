package fullnode

import (
	"context"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPVCControl_Reconcile(t *testing.T) {
	t.Parallel()

	type (
		mockPVCClient = mockClient[*corev1.PersistentVolumeClaim]
		mockPVCDiffer = mockDiffer[*corev1.PersistentVolumeClaim]
	)
	ctx := context.Background()
	const namespace = "testpvc"

	buildPVCs := func(n int) []*corev1.PersistentVolumeClaim {
		return lo.Map(lo.Range(n), func(i int, _ int) *corev1.PersistentVolumeClaim {
			var pvc corev1.PersistentVolumeClaim
			pvc.Name = fmt.Sprintf("pvc-%d", i)
			pvc.Namespace = namespace
			return &pvc
		})
	}

	t.Run("no changes", func(t *testing.T) {
		var mClient mockPVCClient
		mClient.ObjectList = corev1.PersistentVolumeClaimList{
			Items: []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "pvc-1"}},
			},
		}

		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Namespace = namespace
		crd.Name = "hub"

		control := NewPVCControl(&mClient)
		control.diffFactory = func(ordinalAnnotationKey, revisionLabelKey string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			require.Equal(t, "app.kubernetes.io/ordinal", ordinalAnnotationKey)
			require.Equal(t, "app.kubernetes.io/revision", revisionLabelKey)
			require.Len(t, current, 1)
			require.Equal(t, "pvc-1", current[0].Name)
			require.Len(t, want, 3)
			return mockPVCDiffer{}
		}
		requeue, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)
		require.False(t, requeue)

		require.Len(t, mClient.GotListOpts, 3)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, namespace, listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, "app.kubernetes.io/name=hub", listOpt.LabelSelector.String())
		require.Equal(t, ".metadata.controller=hub", listOpt.FieldSelector.String())
	})

	t.Run("scale phase", func(t *testing.T) {
		var (
			mDiff = mockPVCDiffer{
				StubCreates: buildPVCs(3),
				StubDeletes: buildPVCs(2),
				StubUpdates: buildPVCs(10),
			}
			mClient mockPVCClient
			crd     = defaultCRD()
			control = NewPVCControl(&mClient)
		)
		crd.Namespace = namespace
		control.diffFactory = func(_, _ string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return mDiff
		}
		requeue, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 3, mClient.CreateCount)
		require.Equal(t, 2, mClient.DeleteCount)

		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)
	})

	t.Run("updates", func(t *testing.T) {
		updates := buildPVCs(2)
		unbound := updates[0]
		unbound.Status.Phase = corev1.ClaimPending

		var mClient mockPVCClient
		mClient.ObjectList = corev1.PersistentVolumeClaimList{
			Items: []corev1.PersistentVolumeClaim{*unbound},
		}
		var (
			mDiff = mockPVCDiffer{
				StubUpdates: updates,
			}
			crd     = defaultCRD()
			control = NewPVCControl(&mClient)
		)
		crd.Namespace = namespace
		crd.Spec.VolumeClaimTemplate.VolumeMode = ptr(corev1.PersistentVolumeMode("should not be in the patch"))
		control.diffFactory = func(_, _ string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return mDiff
		}
		requeue, rerr := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, rerr)
		require.False(t, requeue)

		require.Empty(t, mClient.CreateCount)
		require.Empty(t, mClient.DeleteCount)

		// Count of 1 because we skip patching unbound claims (results in kube API error).
		require.Equal(t, 1, mClient.PatchCount)
		require.Equal(t, client.Merge, mClient.LastPatch)

		gotPVC := mClient.LastPatchObject.(*corev1.PersistentVolumeClaim)
		require.Empty(t, gotPVC.Spec.VolumeMode)
		require.Equal(t, updates[1].Spec.Resources, gotPVC.Spec.Resources)
	})
}
