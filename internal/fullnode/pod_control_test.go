package fullnode

import (
	"context"
	"fmt"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPodControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockPodClient = mockClient[*corev1.Pod]

	ctx := context.Background()
	const namespace = "test"

	t.Run("no changes", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)
		existing := diff.New(nil, pods).Creates()[0]

		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: []corev1.Pod{*existing},
		}

		syncInfo := &cosmosv1.SyncInfoStatus{
			Pods: []cosmosv1.SyncInfoPodStatus{
				{
					Pod:    "hub-0",
					InSync: ptr(true),
				},
			},
		}

		control := NewPodControl(&mClient)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
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
	})

	t.Run("scale phase", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 3

		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "hub-98"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "hub-99"}},
			},
		}

		control := NewPodControl(&mClient)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, nil)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Equal(t, 3, mClient.CreateCount)
		require.Equal(t, 2, mClient.DeleteCount)

		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)
	})

	t.Run("rollout phase", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 5
		crd.Spec.RolloutStrategy = cosmosv1.RolloutStrategy{
			MaxUnavailable: ptr(intstr.FromInt(2)),
		}

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)
		existing := diff.New(nil, pods).Creates()

		mClient := mockPodClient{
			ObjectList: corev1.PodList{
				Items: valueSlice(existing),
			},
		}

		syncInfo := &cosmosv1.SyncInfoStatus{
			Pods: []cosmosv1.SyncInfoPodStatus{
				{
					Pod:    "hub-0",
					InSync: ptr(true),
				},
				{
					Pod:    "hub-1",
					InSync: ptr(true),
				},
				{
					Pod:    "hub-2",
					InSync: ptr(true),
				},
				{
					Pod:    "hub-3",
					InSync: ptr(true),
				},
				{
					Pod:    "hub-4",
					InSync: ptr(true),
				},
			},
		}

		control := NewPodControl(&mClient)

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 5, ready) // mockPodFilter only returns 1 candidate as ready
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		// Trigger updates
		crd.Spec.PodTemplate.Image = "new-image"
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)

		now := metav1.Now()
		existing[0].DeletionTimestamp = ptr(now)
		existing[1].DeletionTimestamp = ptr(now)

		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 3, ready) // only 3 should be marked ready because 2 are in the deleting state.
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)

		require.True(t, requeue)

		// pod status has not changed, but 0 and 1 are now in deleting state.
		// should not delete any more.
		require.Equal(t, 2, mClient.DeleteCount)

		// once pod deletion is complete, new pods are created with new image.
		existing[0].Spec.Containers[0].Image = "new-image"
		existing[1].Spec.Containers[0].Image = "new-image"
		existing[0].DeletionTimestamp = nil
		existing[1].DeletionTimestamp = nil

		recalculatePodRevision(existing[0], 0)
		recalculatePodRevision(existing[1], 1)
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		syncInfo.Pods[0].InSync = nil
		syncInfo.Pods[0].Error = ptr("upgrade in progress")

		syncInfo.Pods[1].InSync = nil
		syncInfo.Pods[1].Error = ptr("upgrade in progress")

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 3, ready)
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)

		// should not delete any more yet.
		require.Equal(t, 2, mClient.DeleteCount)
	})

	t.Run("rollout version upgrade rolling", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 5
		crd.Spec.RolloutStrategy = cosmosv1.RolloutStrategy{
			MaxUnavailable: ptr(intstr.FromInt(2)),
		}
		crd.Spec.ChainSpec = cosmosv1.ChainSpec{
			Versions: []cosmosv1.ChainVersion{
				{
					Image: "image",
				},
				{
					UpgradeHeight: 100,
					Image:         "new-image",
				},
			},
		}
		crd.Status.Height = make(map[string]uint64)

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)
		existing := diff.New(nil, pods).Creates()

		mClient := mockPodClient{
			ObjectList: corev1.PodList{
				Items: valueSlice(existing),
			},
		}

		// pods are at upgrade height and reachable
		syncInfo := &cosmosv1.SyncInfoStatus{
			Pods: []cosmosv1.SyncInfoPodStatus{
				{
					Pod:    "hub-0",
					Height: ptr(uint64(100)),
					InSync: ptr(true),
				},
				{
					Pod:    "hub-1",
					Height: ptr(uint64(100)),
					InSync: ptr(true),
				},
				{
					Pod:    "hub-2",
					Height: ptr(uint64(100)),
					InSync: ptr(true),
				},
				{
					Pod:    "hub-3",
					Height: ptr(uint64(100)),
					InSync: ptr(true),
				},
				{
					Pod:    "hub-4",
					Height: ptr(uint64(100)),
					InSync: ptr(true),
				},
			},
		}

		control := NewPodControl(&mClient)

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 5, ready) // all are reachable and reporting ready, so we will maintain liveliness.
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		// Trigger updates
		for _, pod := range existing {
			crd.Status.Height[pod.Name] = 100
		}

		// Reconcile 1, should update 0 and 1

		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)

		// only handled 2 updates, so should requeue.
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, 2, mClient.DeleteCount)

		now := metav1.Now()
		existing[0].DeletionTimestamp = ptr(now)
		existing[1].DeletionTimestamp = ptr(now)

		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 3, ready) // only 3 should be marked ready because 2 are in the deleting state.
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)

		require.True(t, requeue)

		// pod status has not changed, but 0 and 1 are now in deleting state.
		// should not delete any more.
		require.Equal(t, 2, mClient.DeleteCount)

		existing[0].Spec.Containers[0].Image = "new-image"
		existing[1].Spec.Containers[0].Image = "new-image"
		existing[0].DeletionTimestamp = nil
		existing[1].DeletionTimestamp = nil

		recalculatePodRevision(existing[0], 0)
		recalculatePodRevision(existing[1], 1)
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		// 0 and 1 are now unavailable, working on upgrade
		syncInfo.Pods[0].InSync = nil
		syncInfo.Pods[0].Error = ptr("upgrade in progress")

		syncInfo.Pods[1].InSync = nil
		syncInfo.Pods[1].Error = ptr("upgrade in progress")

		// Reconcile 2, should not update anything because 0 and 1 are still in progress.

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 3, ready)
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)

		// no further updates yet, should requeue.
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)

		// should not delete any more yet.
		require.Equal(t, 2, mClient.DeleteCount)

		// mock out that one of the pods completed the upgrade. should begin upgrading one more
		syncInfo.Pods[0].InSync = ptr(true)
		syncInfo.Pods[0].Height = ptr(uint64(101))
		syncInfo.Pods[0].Error = nil

		// Reconcile 3, should update pod 2 (only one) because 1 is still in progress, but 0 is done.

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 4, ready)
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)

		// only handled 1 updates, so should requeue.
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)

		// should delete one more
		require.Equal(t, 3, mClient.DeleteCount)

		now = metav1.Now()
		existing[2].DeletionTimestamp = ptr(now)

		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 3, ready) // only 3 should be marked ready because 2 is in the deleting state and 1 is still in progress upgrading.
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)

		require.True(t, requeue)

		// pod status has not changed, but 2 is now in deleting state.
		// should not delete any more.
		require.Equal(t, 3, mClient.DeleteCount)

		existing[2].Spec.Containers[0].Image = "new-image"
		existing[2].DeletionTimestamp = nil
		recalculatePodRevision(existing[2], 2)
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		// mock out that both pods completed the upgrade. should begin upgrading the last 2
		syncInfo.Pods[1].InSync = ptr(true)
		syncInfo.Pods[1].Height = ptr(uint64(101))
		syncInfo.Pods[1].Error = nil

		syncInfo.Pods[2].InSync = ptr(true)
		syncInfo.Pods[2].Height = ptr(uint64(101))
		syncInfo.Pods[2].Error = nil

		// Reconcile 4, should update 3 and 4 because the rest are done.

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 5, ready)
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)

		// all updates are now handled, no longer need requeue.
		require.False(t, requeue)

		require.Zero(t, mClient.CreateCount)

		// should delete the last 2
		require.Equal(t, 5, mClient.DeleteCount)
	})

	t.Run("rollout version upgrade halt", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 5
		crd.Spec.RolloutStrategy = cosmosv1.RolloutStrategy{
			MaxUnavailable: ptr(intstr.FromInt(2)),
		}
		crd.Spec.ChainSpec = cosmosv1.ChainSpec{
			Versions: []cosmosv1.ChainVersion{
				{
					Image: "image",
				},
				{
					UpgradeHeight: 100,
					Image:         "new-image",
					SetHaltHeight: true,
				},
			},
		}
		crd.Status.Height = make(map[string]uint64)

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)
		existing := diff.New(nil, pods).Creates()

		mClient := mockPodClient{
			ObjectList: corev1.PodList{
				Items: valueSlice(existing),
			},
		}

		// pods are at upgrade height and reachable
		syncInfo := &cosmosv1.SyncInfoStatus{
			Pods: []cosmosv1.SyncInfoPodStatus{
				{
					Pod:    "hub-0",
					Height: ptr(uint64(100)),
					Error:  ptr("panic at upgrade height"),
				},
				{
					Pod:    "hub-1",
					Height: ptr(uint64(100)),
					Error:  ptr("panic at upgrade height"),
				},
				{
					Pod:    "hub-2",
					Height: ptr(uint64(100)),
					Error:  ptr("panic at upgrade height"),
				},
				{
					Pod:    "hub-3",
					Height: ptr(uint64(100)),
					Error:  ptr("panic at upgrade height"),
				},
				{
					Pod:    "hub-4",
					Height: ptr(uint64(100)),
					Error:  ptr("panic at upgrade height"),
				},
			},
		}

		control := NewPodControl(&mClient)

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 0, ready) // mockPodFilter returns no pods as synced, but all are at the upgrade height.
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		// Trigger updates
		for _, pod := range existing {
			crd.Status.Height[pod.Name] = 100
		}

		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)

		// all updates are handled, so should not requeue
		require.False(t, requeue)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, 5, mClient.DeleteCount)
	})
}

// revision hash must be taken without the revision label and the ordinal annotation.
func recalculatePodRevision(pod *corev1.Pod, ordinal int) {
	delete(pod.Labels, "app.kubernetes.io/revision")
	delete(pod.Annotations, "app.kubernetes.io/ordinal")
	rev1 := diff.Adapt(pod, ordinal).Revision()
	pod.Labels["app.kubernetes.io/revision"] = rev1
	pod.Annotations["app.kubernetes.io/ordinal"] = fmt.Sprintf("%d", ordinal)
}
