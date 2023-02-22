package fullnode

import (
	"context"
	"errors"
	"fmt"
	"testing"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/strangelove-ventures/cosmos-operator/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var nopReporter test.NopReporter

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

	testPVCControl := func(client Client) PVCControl {
		control := NewPVCControl(client)
		control.recentVolumeSnapshot = func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error) {
			panic("recentVolumeSnapshot should not be called")
		}
		return control
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

		control := testPVCControl(&mClient)
		control.diffFactory = func(ordinalAnnotationKey, revisionLabelKey string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			require.Equal(t, "app.kubernetes.io/ordinal", ordinalAnnotationKey)
			require.Equal(t, "app.kubernetes.io/revision", revisionLabelKey)
			require.Len(t, current, 1)
			require.Equal(t, "pvc-1", current[0].Name)
			require.Len(t, want, 3)
			return mockPVCDiffer{}
		}

		requeue, err := control.Reconcile(ctx, nopReporter, &crd)
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
			control = testPVCControl(&mClient)
		)
		crd.Namespace = namespace
		control.diffFactory = func(_, _ string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return mDiff
		}
		requeue, err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 3, mClient.CreateCount)
		require.Equal(t, 2, mClient.DeleteCount)

		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)
	})

	t.Run("create - autoDataSource", func(t *testing.T) {
		pvcs := buildPVCs(5)
		pvcs[0].Spec.DataSource = &corev1.TypedLocalObjectReference{Name: "existing1"}
		pvcs[1].Spec.DataSourceRef = &corev1.TypedLocalObjectReference{Name: "existing2"}

		var (
			mClient mockPVCClient
			crd     = defaultCRD()
			control = testPVCControl(&mClient)
		)
		crd.Namespace = namespace
		crd.Spec.VolumeClaimTemplate.AutoDataSource = &cosmosv1.AutoDataSource{
			VolumeSnapshotSelector: map[string]string{"label": "vol-snapshot"},
		}
		control.diffFactory = func(_, _ string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return mockPVCDiffer{StubCreates: pvcs}
		}
		var volCallCount int
		control.recentVolumeSnapshot = func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error) {
			require.NotNil(t, ctx)
			require.Equal(t, &mClient, lister)
			require.Equal(t, "testpvc", namespace)
			require.Equal(t, map[string]string{"label": "vol-snapshot"}, selector)
			var stub snapshotv1.VolumeSnapshot
			stub.Name = "found-snapshot"
			volCallCount++
			return &stub, nil
		}
		requeue, err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 1, volCallCount)
		require.Equal(t, 5, len(mClient.CreatedObjects))

		require.Equal(t, pvcs[0].Spec.DataSource, mClient.CreatedObjects[0].Spec.DataSource)
		require.Equal(t, pvcs[1].Spec.DataSource, mClient.CreatedObjects[1].Spec.DataSource)

		want := corev1.TypedLocalObjectReference{
			APIGroup: ptr("snapshot.storage.k8s.io"),
			Kind:     "VolumeSnapshot",
			Name:     "found-snapshot",
		}
		for _, pvc := range mClient.CreatedObjects[2:] {
			ds := pvc.Spec.DataSource
			require.NotNil(t, ds)
			require.Equal(t, want, *ds)
		}
	})

	t.Run("create - autoDataSource error", func(t *testing.T) {
		var (
			mClient mockPVCClient
			crd     = defaultCRD()
			control = testPVCControl(&mClient)
		)
		crd.Namespace = namespace
		crd.Spec.VolumeClaimTemplate.AutoDataSource = &cosmosv1.AutoDataSource{
			VolumeSnapshotSelector: map[string]string{"label": "vol-snapshot"},
		}
		control.diffFactory = func(_, _ string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return mockPVCDiffer{StubCreates: buildPVCs(1)}
		}
		var volCallCount int
		control.recentVolumeSnapshot = func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error) {
			volCallCount++
			return nil, errors.New("boom")
		}
		requeue, err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 1, mClient.CreateCount)
		require.Equal(t, 1, volCallCount)

		require.Nil(t, mClient.LastCreateObject.Spec.DataSource)
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
			control = testPVCControl(&mClient)
		)
		crd.Namespace = namespace
		crd.Spec.VolumeClaimTemplate.VolumeMode = ptr(corev1.PersistentVolumeMode("should not be in the patch"))
		control.diffFactory = func(_, _ string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return mDiff
		}
		requeue, rerr := control.Reconcile(ctx, nopReporter, &crd)
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

	t.Run("retention policy", func(t *testing.T) {
		var (
			mDiff = mockPVCDiffer{
				StubDeletes: buildPVCs(2),
			}
			mClient mockPVCClient
			crd     = defaultCRD()
			control = testPVCControl(&mClient)
		)
		crd.Namespace = namespace
		crd.Spec.RetentionPolicy = ptr(cosmosv1.RetentionPolicyRetain)
		control.diffFactory = func(_, _ string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return mDiff
		}
		requeue, err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)

		require.Zero(t, mClient.DeleteCount)
		require.False(t, requeue)

		crd.Spec.RetentionPolicy = ptr(cosmosv1.RetentionPolicyDelete)
		requeue, err = control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)

		require.Equal(t, 2, mClient.DeleteCount)
		require.True(t, requeue)
	})
}
