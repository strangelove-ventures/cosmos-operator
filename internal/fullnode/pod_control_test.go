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

type mockPodClient struct{ mockClient[*corev1.Pod] }

func newMockPodClient(pods []*corev1.Pod) *mockPodClient {
	return &mockPodClient{
		mockClient: mockClient[*corev1.Pod]{
			ObjectList: corev1.PodList{
				Items: valueSlice(pods),
			},
		},
	}
}

func (c *mockPodClient) setPods(pods []*corev1.Pod) {
	c.ObjectList = corev1.PodList{
		Items: valueSlice(pods),
	}
}

func (c *mockPodClient) upgradePods(
	t *testing.T,
	crdName string,
	ordinals ...int,
) {
	existing := ptrSlice(c.ObjectList.(corev1.PodList).Items)
	for _, ordinal := range ordinals {
		updatePod(t, crdName, ordinal, existing, newPodWithNewImage, true)
	}
	c.setPods(existing)
}

func (c *mockPodClient) deletePods(
	t *testing.T,
	crdName string,
	ordinals ...int,
) {
	existing := ptrSlice(c.ObjectList.(corev1.PodList).Items)
	for _, ordinal := range ordinals {
		updatePod(t, crdName, ordinal, existing, deletedPod, false)
	}
	c.setPods(existing)
}

func TestPodControl_Reconcile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const namespace = "test"

	t.Run("no changes", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)
		existing := diff.New(nil, pods).Creates()

		require.Len(t, existing, 1)

		mClient := newMockPodClient(existing)

		syncInfo := map[string]*cosmosv1.SyncInfoPodStatus{
			"hub-0": {InSync: ptr(true)},
		}

		control := NewPodControl(mClient, nil)
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

	t.Run("no changes with additional pods", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1
		crd.Spec.AdditionalVersionedPods = []cosmosv1.AdditionalPodSpec{
			{
				Name: "metrics",
				PodSpec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "metrics",
							Image: "metrics-image:v1",
						},
					},
				},
			},
		}

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)
		existing := diff.New(nil, pods).Creates()

		// Should have 2 pods - the main pod and the additional pod
		require.Len(t, existing, 2)

		mClient := newMockPodClient(existing)

		syncInfo := map[string]*cosmosv1.SyncInfoPodStatus{
			"hub-0": {InSync: ptr(true)},
		}

		control := NewPodControl(mClient, nil)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)
		require.False(t, requeue)

		require.Len(t, mClient.GotListOpts, 2)
	})

	t.Run("scale phase", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 3

		mClient := newMockPodClient([]*corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "hub-98"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "hub-99"}},
		})

		control := NewPodControl(mClient, nil)
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

	t.Run("scale phase with additional pods", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 3
		crd.Spec.AdditionalVersionedPods = []cosmosv1.AdditionalPodSpec{
			{
				Name: "metrics",
				PodSpec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "metrics",
							Image: "metrics-image:v1",
						},
					},
				},
			},
		}

		mClient := newMockPodClient([]*corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "hub-98"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "hub-99"}},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metrics-98",
					Labels: map[string]string{
						kube.BelongsToLabel: "hub-98",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metrics-99",
					Labels: map[string]string{
						kube.BelongsToLabel: "hub-99",
					},
				},
			},
		})

		control := NewPodControl(mClient, nil)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, nil)
		require.NoError(t, err)
		require.True(t, requeue)

		// 3 main pods + 3 additional pods = 6 creates
		require.Equal(t, 6, mClient.CreateCount)
		// 2 main pods + 2 additional pods = 4 deletes
		require.Equal(t, 4, mClient.DeleteCount)

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

		mClient := newMockPodClient(diff.New(nil, pods).Creates())

		syncInfo := map[string]*cosmosv1.SyncInfoPodStatus{
			"hub-0": {InSync: ptr(true)},
			"hub-1": {InSync: ptr(true)},
			"hub-2": {InSync: ptr(true)},
			"hub-3": {InSync: ptr(true)},
			"hub-4": {InSync: ptr(true)},
		}

		control := NewPodControl(mClient, nil)

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

		mClient.deletePods(t, crd.Name, 0, 1)

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
		mClient.upgradePods(t, crd.Name, 0, 1)

		syncInfo["hub-0"].InSync = nil
		syncInfo["hub-0"].Error = ptr("upgrade in progress")

		syncInfo["hub-1"].InSync = nil
		syncInfo["hub-1"].Error = ptr("upgrade in progress")

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

	t.Run("rollout phase with additional pods", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 5
		crd.Spec.RolloutStrategy = cosmosv1.RolloutStrategy{
			MaxUnavailable: ptr(intstr.FromInt(2)),
		}
		crd.Spec.AdditionalVersionedPods = []cosmosv1.AdditionalPodSpec{
			{
				Name: "metrics",
				PodSpec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "metrics",
							Image: "metrics-image:v1",
						},
					},
				},
			},
		}

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)

		existing := diff.New(nil, pods).Creates()

		// Add labels to identify additional pods
		for i, pod := range existing {
			if i >= 5 { // First 5 are main pods, rest are additional
				pod.Labels[kube.BelongsToLabel] = fmt.Sprintf("hub-%d", i-5)
			}
		}

		// Create a fresh mock client with the existing pods
		mClient := newMockPodClient(existing)
		mClient.DeleteCount = 0 // Reset delete count to be sure

		syncInfo := map[string]*cosmosv1.SyncInfoPodStatus{
			"hub-0": {InSync: ptr(true)},
			"hub-1": {InSync: ptr(true)},
			"hub-2": {InSync: ptr(true)},
			"hub-3": {InSync: ptr(true)},
			"hub-4": {InSync: ptr(true)},
		}

		control := NewPodControl(mClient, nil)

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 5, ready)
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		// PHASE 1: Trigger updates for all pods (main and additional)
		t.Log("Phase 1: Initial update with image changes for all pods")
		crd.Spec.PodTemplate.Image = "new-image"
		crd.Spec.AdditionalVersionedPods[0].PodSpec.Containers[0].Image = "metrics-image:v2"

		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)

		// With these changes, we expect:
		// - 2 main pods deleted (based on maxUnavailable=2)
		// - All 5 additional pods deleted (since they all have image changes)
		require.Equal(t, 7, mClient.DeleteCount, "Expected 2 main pods + 5 additional pods to be deleted")

		// Reset the mock client for the next phase
		// Create a new list with the deleted pods removed
		var remainingPods []*corev1.Pod
		for _, pod := range existing {
			// Keep pods that weren't deleted (hub-2, hub-3, hub-4)
			if pod.Labels[kube.BelongsToLabel] == "" && pod.Name != "hub-0" && pod.Name != "hub-1" {
				remainingPods = append(remainingPods, pod)
			}
		}

		mClient = newMockPodClient(remainingPods)
		control = NewPodControl(mClient, nil)

		// Update syncInfo to reflect deleted pods
		syncInfo["hub-0"] = &cosmosv1.SyncInfoPodStatus{
			InSync: nil,
			Error:  ptr("upgrade in progress"),
		}
		syncInfo["hub-1"] = &cosmosv1.SyncInfoPodStatus{
			InSync: nil,
			Error:  ptr("upgrade in progress"),
		}

		// PHASE 2: Continue rollout with remaining main pods
		t.Log("Phase 2: Continue rollout with remaining pods")
		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 3, ready) // 3 main pods still ready
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)
		require.True(t, requeue)

		// No more pods should be deleted - we're at maxUnavailable
		require.Equal(t, 0, mClient.DeleteCount, "No more pods should be deleted at this point")

		// PHASE 3: Simulate completed upgrade of first pods
		t.Log("Phase 3: First pods complete upgrade")
		syncInfo["hub-0"] = &cosmosv1.SyncInfoPodStatus{
			InSync: ptr(true),
			Error:  nil,
		}

		// Add the upgraded pods back to the list
		upgradedMainPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "hub-0",
				Labels: map[string]string{},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "node",
						Image: "new-image", // Updated image
					},
				},
			},
		}

		// Update the client with the new pod state
		remainingPods = append(remainingPods, upgradedMainPod)
		mClient = newMockPodClient(remainingPods)
		control = NewPodControl(mClient, nil)

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 4, ready) // 4 main pods now ready
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)
		require.True(t, requeue)

		// Should delete one more main pod
		require.Equal(t, 1, mClient.DeleteCount, "Should delete one more main pod")
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

		mClient := newMockPodClient(existing)

		// pods are at upgrade height and reachable
		syncInfo := map[string]*cosmosv1.SyncInfoPodStatus{
			"hub-0": {
				Height: ptr(uint64(100)),
				InSync: ptr(true),
			},
			"hub-1": {
				Height: ptr(uint64(100)),
				InSync: ptr(true),
			},
			"hub-2": {
				Height: ptr(uint64(100)),
				InSync: ptr(true),
			},
			"hub-3": {
				Height: ptr(uint64(100)),
				InSync: ptr(true),
			},
			"hub-4": {
				Height: ptr(uint64(100)),
				InSync: ptr(true),
			},
		}

		control := NewPodControl(mClient, nil)

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

		mClient.deletePods(t, crd.Name, 0, 1)

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

		mClient.upgradePods(t, crd.Name, 0, 1)

		// 0 and 1 are now unavailable, working on upgrade
		syncInfo["hub-0"].InSync = nil
		syncInfo["hub-0"].Error = ptr("upgrade in progress")

		syncInfo["hub-1"].InSync = nil
		syncInfo["hub-1"].Error = ptr("upgrade in progress")

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
		syncInfo["hub-0"].InSync = ptr(true)
		syncInfo["hub-0"].Height = ptr(uint64(101))
		syncInfo["hub-0"].Error = nil

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

		mClient.deletePods(t, crd.Name, 2)

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

		mClient.upgradePods(t, crd.Name, 2)

		// mock out that both pods completed the upgrade. should begin upgrading the last 2
		syncInfo["hub-1"].InSync = ptr(true)
		syncInfo["hub-1"].Height = ptr(uint64(101))
		syncInfo["hub-1"].Error = nil

		syncInfo["hub-2"].InSync = ptr(true)
		syncInfo["hub-2"].Height = ptr(uint64(101))
		syncInfo["hub-2"].Error = nil

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

	t.Run("version mismatch with unreachable pod", func(t *testing.T) {
		// Create a simple CRD with upgrade heights
		crd := defaultCRD()
		crd.Name = "test-version-mismatch"
		crd.Namespace = namespace
		crd.Spec.Replicas = 2

		// Configure version upgrade
		crd.Spec.ChainSpec = cosmosv1.ChainSpec{
			Binary: "gaiad",
			Versions: []cosmosv1.ChainVersion{
				{
					Image: "cosmos:v1",
				},
				{
					UpgradeHeight: 100,
					Image:         "cosmos:v2",
				},
			},
		}

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)

		// Set up heights for both pods
		crd.Status.Height = map[string]uint64{
			"test-version-mismatch-0": 100,
			"test-version-mismatch-1": 100,
		}

		// Create sync info - pod 0 is reachable, pod 1 is unreachable
		syncInfo := map[string]*cosmosv1.SyncInfoPodStatus{
			"test-version-mismatch-0": {
				InSync: ptr(true),
				Error:  nil,
				Height: ptr(uint64(100)),
			},
			"test-version-mismatch-1": {
				InSync: nil,
				Error:  ptr("unreachable"),
				Height: ptr(uint64(100)),
			},
		}

		// Create client and controller
		mClient := newMockPodClient(diff.New(nil, pods).Creates())
		control := NewPodControl(mClient, nil)

		// Set computeRollout to allow 1 pod to be deleted
		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			return 1 // Allow 1 pod to be deleted
		}

		// Run reconcile
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil, syncInfo)
		require.NoError(t, err)
		require.True(t, requeue)

		// The key test - only the unreachable pod should be deleted
		require.Equal(t, 1, mClient.DeleteCount, "Unreachable pod with version mismatch should be deleted")
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

		mClient := newMockPodClient(existing)

		// pods are at upgrade height and reachable
		syncInfo := map[string]*cosmosv1.SyncInfoPodStatus{
			"hub-0": {
				Height: ptr(uint64(100)),
				Error:  ptr("panic at upgrade height"),
			},
			"hub-1": {
				Height: ptr(uint64(100)),
				Error:  ptr("panic at upgrade height"),
			},
			"hub-2": {
				Height: ptr(uint64(100)),
				Error:  ptr("panic at upgrade height"),
			},
			"hub-3": {
				Height: ptr(uint64(100)),
				Error:  ptr("panic at upgrade height"),
			},
			"hub-4": {
				Height: ptr(uint64(100)),
				Error:  ptr("panic at upgrade height"),
			},
		}

		control := NewPodControl(mClient, nil)

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

func newPodWithNewImage(pod *corev1.Pod) {
	pod.DeletionTimestamp = nil
	pod.Spec.Containers[0].Image = "new-image"
	if len(pod.Spec.InitContainers) > 1 {
		pod.Spec.InitContainers[1].Image = "new-image"
	}
}

func deletedPod(pod *corev1.Pod) {
	pod.DeletionTimestamp = ptr(metav1.Now())
}

func updatePod(t *testing.T, crdName string, ordinal int, pods []*corev1.Pod, updateFn func(pod *corev1.Pod), recalc bool) {
	podName := fmt.Sprintf("%s-%d", crdName, ordinal)
	for _, pod := range pods {
		if pod.Name == podName {
			updateFn(pod)
			if recalc {
				recalculatePodRevision(pod, ordinal)
			}
			return
		}
	}

	require.FailNow(t, "pod not found", podName)
}
