package fullnode

import (
	"context"
	"errors"
	"testing"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/strangelove-ventures/cosmos-operator/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var nopReporter test.NopReporter

func TestPVCControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockPVCClient = mockClient[*corev1.PersistentVolumeClaim]

	ctx := context.Background()
	const namespace = "test"

	testPVCControl := func(client Client) PVCControl {
		control := NewPVCControl(client)
		control.recentVolumeSnapshot = func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error) {
			panic("recentVolumeSnapshot should not be called")
		}
		return control
	}

	t.Run("no changes", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1
		existing := diff.New(nil, BuildPVCs(&crd, map[int32]*dataSource{}, nil)).Creates()[0]
		existing.Status.Phase = corev1.ClaimBound

		var mClient mockPVCClient
		mClient.ObjectList = corev1.PersistentVolumeClaimList{
			Items: []corev1.PersistentVolumeClaim{
				*existing,
			},
		}

		control := testPVCControl(&mClient)

		requeue, err := control.Reconcile(ctx, nopReporter, &crd, &PVCStatusChanges{})
		require.NoError(t, err)
		require.False(t, requeue)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, namespace, listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=hub", listOpt.FieldSelector.String())

		require.Empty(t, mClient.LastPatchObject)
	})

	t.Run("scale phase", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = namespace
		crd.Name = "hub"
		crd.Spec.Replicas = 1
		existing := BuildPVCs(&crd, map[int32]*dataSource{}, nil)[0].Object()

		var mClient mockPVCClient
		mClient.ObjectList = corev1.PersistentVolumeClaimList{
			Items: []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "pvc-hub-98", Namespace: namespace}}, // delete
				{ObjectMeta: metav1.ObjectMeta{Name: "pvc-hub-99", Namespace: namespace}}, // delete
				*existing,
			},
		}

		crd.Spec.Replicas = 4
		control := testPVCControl(&mClient)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, &PVCStatusChanges{})
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 3, mClient.CreateCount)
		require.Equal(t, 2, mClient.DeleteCount)
		require.Zero(t, mClient.UpdateCount)

		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)
	})

	t.Run("create - autoDataSource", func(t *testing.T) {
		var (
			mClient mockPVCClient
			crd     = defaultCRD()
			control = testPVCControl(&mClient)
		)
		crd.Namespace = namespace
		crd.Spec.Replicas = 3
		crd.Spec.VolumeClaimTemplate.AutoDataSource = &cosmosv1.AutoDataSource{
			VolumeSnapshotSelector: map[string]string{"label": "vol-snapshot"},
		}

		var volCallCount int
		control.recentVolumeSnapshot = func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error) {
			require.NotNil(t, ctx)
			require.Equal(t, &mClient, lister)
			require.Equal(t, namespace, namespace)
			require.Equal(t, map[string]string{"label": "vol-snapshot"}, selector)
			var stub snapshotv1.VolumeSnapshot
			stub.Name = "found-snapshot"
			stub.Status = &snapshotv1.VolumeSnapshotStatus{
				ReadyToUse:  ptr(true),
				RestoreSize: ptr(resource.MustParse("100Gi")),
			}
			volCallCount++
			return &stub, nil
		}
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, &PVCStatusChanges{})
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 3, volCallCount)
		require.Equal(t, 3, mClient.CreateCount)

		want := corev1.TypedLocalObjectReference{
			APIGroup: ptr("snapshot.storage.k8s.io"),
			Kind:     "VolumeSnapshot",
			Name:     "found-snapshot",
		}
		for _, pvc := range mClient.CreatedObjects {
			ds := pvc.Spec.DataSource
			require.NotNil(t, ds)
			require.Equal(t, want, *ds)
		}
	})

	t.Run("create - autoDataSource dataSource already set", func(t *testing.T) {
		var (
			mClient mockPVCClient
			crd     = defaultCRD()
			control = testPVCControl(&mClient)
		)
		crdDataSource := &corev1.TypedLocalObjectReference{
			APIGroup: ptr("snapshot.storage.k8s.io"),
			Kind:     "VolumeSnapshot",
			Name:     "user-set-snapshot",
		}
		crd.Namespace = namespace
		crd.Spec.Replicas = 2
		crd.Spec.VolumeClaimTemplate.AutoDataSource = &cosmosv1.AutoDataSource{
			VolumeSnapshotSelector: map[string]string{"label": "vol-snapshot"},
		}
		crd.Spec.VolumeClaimTemplate.DataSource = crdDataSource

		control.recentVolumeSnapshot = func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error) {
			panic("should not be called")
		}

		mClient.Object = snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-set-snapshot",
				Namespace: namespace,
			},
			Status: &snapshotv1.VolumeSnapshotStatus{
				ReadyToUse:  ptr(true),
				RestoreSize: ptr(resource.MustParse("100Gi")),
			},
		}

		requeue, err := control.Reconcile(ctx, nopReporter, &crd, &PVCStatusChanges{})
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 2, mClient.CreateCount)

		for _, pvc := range mClient.CreatedObjects {
			ds := pvc.Spec.DataSource
			require.NotNil(t, ds)
			require.Equal(t, *crdDataSource, *ds)
		}
	})

	t.Run("create - autoDataSource error", func(t *testing.T) {
		var (
			mClient mockPVCClient
			crd     = defaultCRD()
			control = testPVCControl(&mClient)
		)
		crd.Namespace = namespace
		crd.Spec.Replicas = 1
		crd.Spec.VolumeClaimTemplate.AutoDataSource = &cosmosv1.AutoDataSource{
			VolumeSnapshotSelector: map[string]string{"label": "vol-snapshot"},
		}
		var volCallCount int
		control.recentVolumeSnapshot = func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error) {
			volCallCount++
			return nil, errors.New("boom")
		}
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, &PVCStatusChanges{})
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 1, mClient.CreateCount)
		require.Equal(t, 1, volCallCount)

		require.Nil(t, mClient.LastCreateObject.Spec.DataSource)
	})

	t.Run("updates", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1

		var mClient mockPVCClient
		existing := BuildPVCs(&crd, map[int32]*dataSource{}, nil)[0].Object()
		existing.Status.Phase = corev1.ClaimBound
		mClient.ObjectList = corev1.PersistentVolumeClaimList{
			Items: []corev1.PersistentVolumeClaim{*existing},
		}

		// Cause a change
		crd.Spec.VolumeClaimTemplate.VolumeMode = ptr(corev1.PersistentVolumeMode("should not be in the patch"))
		crd.Spec.VolumeClaimTemplate.Resources.Requests["memory"] = resource.MustParse("1Gi")

		control := testPVCControl(&mClient)
		requeue, rerr := control.Reconcile(ctx, nopReporter, &crd, &PVCStatusChanges{})
		require.NoError(t, rerr)
		require.False(t, requeue)

		require.Empty(t, mClient.CreateCount)
		require.Empty(t, mClient.DeleteCount)

		require.Equal(t, 1, mClient.PatchCount)
		require.Equal(t, client.Merge, mClient.LastPatch)

		gotPatch := mClient.LastPatchObject.(*corev1.PersistentVolumeClaim)
		require.Equal(t, existing.Name, gotPatch.Name)
		require.Equal(t, namespace, gotPatch.Namespace)
		require.Empty(t, gotPatch.Spec.VolumeMode)
		require.Equal(t, crd.Spec.VolumeClaimTemplate.Resources, gotPatch.Spec.Resources)
	})

	t.Run("updates with unbound volumes", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1

		existing := BuildPVCs(&crd, map[int32]*dataSource{}, nil)[0].Object()
		existing.Status.Phase = corev1.ClaimPending
		var mClient mockPVCClient
		mClient.ObjectList = corev1.PersistentVolumeClaimList{
			Items: []corev1.PersistentVolumeClaim{*existing},
		}

		// Cause a change
		crd.Spec.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1Ti")

		control := testPVCControl(&mClient)
		requeue, rerr := control.Reconcile(ctx, nopReporter, &crd, &PVCStatusChanges{})
		require.NoError(t, rerr)
		require.True(t, requeue)

		require.Zero(t, mClient.PatchCount)
	})

	t.Run("retention policy", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = namespace
		crd.Spec.Replicas = 0
		crd.Spec.RetentionPolicy = ptr(cosmosv1.RetentionPolicyRetain)

		var mClient mockPVCClient
		mClient.ObjectList = corev1.PersistentVolumeClaimList{
			Items: []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "pvc-hub-98", Namespace: namespace}}, // delete
				{ObjectMeta: metav1.ObjectMeta{Name: "pvc-hub-99", Namespace: namespace}}, // delete
			},
		}

		control := testPVCControl(&mClient)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, &PVCStatusChanges{})
		require.NoError(t, err)
		require.False(t, requeue)

		require.Zero(t, mClient.DeleteCount)
	})
}
