package fullnode

import (
	"context"
	"testing"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockPodFilter func(ctx context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus

func (fn mockPodFilter) PodsWithStatus(ctx context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
	if ctx == nil {
		panic("nil context")
	}
	return fn(ctx, crd)
}

var panicPodFilter = mockPodFilter(func(context.Context, *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
	panic("SyncedPods should not be called")
})

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

		control := NewPodControl(&mClient, panicPodFilter)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil)
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

		control := NewPodControl(&mClient, panicPodFilter)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil)
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

		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		var didFilter bool
		podFilter := mockPodFilter(func(_ context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
			require.Equal(t, namespace, crd.Namespace)
			require.Equal(t, "hub", crd.Name)
			didFilter = true
			return lo.Map(existing, func(pod *corev1.Pod, i int) cosmos.PodStatus {
				return cosmos.PodStatus{
					Pod:          pod,
					RPCReachable: true,
					Synced:       true,
				}
			})
		})

		control := NewPodControl(&mClient, podFilter)
		const stubRollout = 5

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, stubRollout, ready) // mockPodFilter only returns 1 candidate as ready
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		// Trigger updates
		crd.Spec.PodTemplate.Image = "new-image"
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)
		require.True(t, requeue)

		require.True(t, didFilter)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, 2, mClient.DeleteCount)

		didFilter = false
		podFilter = mockPodFilter(func(_ context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
			require.Equal(t, namespace, crd.Namespace)
			require.Equal(t, "hub", crd.Name)
			didFilter = true
			return lo.Map(existing, func(pod *corev1.Pod, i int) cosmos.PodStatus {
				ps := cosmos.PodStatus{
					Pod:          pod,
					RPCReachable: true,
					Synced:       true,
				}
				if i < 2 {
					ps.RPCReachable = false
					ps.Synced = false
				}
				return ps
			})
		})

		control = NewPodControl(&mClient, podFilter)

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)
		require.True(t, requeue)

		require.True(t, didFilter)

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

		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		var didFilter bool
		podFilter := mockPodFilter(func(_ context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
			require.Equal(t, namespace, crd.Namespace)
			require.Equal(t, "hub", crd.Name)
			didFilter = true
			return lo.Map(existing, func(pod *corev1.Pod, i int) cosmos.PodStatus {
				return cosmos.PodStatus{
					Pod: pod,
					// pods are at or above upgrade height and not reachable
					AwaitingUpgrade: true,
					RPCReachable:    true,
					Synced:          false,
				}
			})
		})

		control := NewPodControl(&mClient, podFilter)

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

		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)

		// only handled 2 updates, so should requeue.
		require.True(t, requeue)

		require.True(t, didFilter)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, 2, mClient.DeleteCount)

		// revision hash must be taken without the revision label and the ordinal annotation.
		existing[0].Spec.Containers[0].Image = "new-image"
		delete(existing[0].Labels, "app.kubernetes.io/revision")
		delete(existing[0].Annotations, "app.kubernetes.io/ordinal")
		rev0 := diff.Adapt(existing[0], 0).Revision()
		existing[0].Labels["app.kubernetes.io/revision"] = rev0
		existing[0].Annotations["app.kubernetes.io/ordinal"] = "0"

		existing[1].Spec.Containers[0].Image = "new-image"
		delete(existing[1].Labels, "app.kubernetes.io/revision")
		delete(existing[1].Annotations, "app.kubernetes.io/ordinal")
		rev1 := diff.Adapt(existing[1], 1).Revision()
		existing[1].Labels["app.kubernetes.io/revision"] = rev1
		existing[1].Annotations["app.kubernetes.io/ordinal"] = "1"
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		// 2 are now unavailable, working on upgrade

		didFilter = false
		podFilter = mockPodFilter(func(_ context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
			require.Equal(t, namespace, crd.Namespace)
			require.Equal(t, "hub", crd.Name)
			didFilter = true
			return lo.Map(existing, func(pod *corev1.Pod, i int) cosmos.PodStatus {
				ps := cosmos.PodStatus{
					Pod:          pod,
					RPCReachable: true,
					Synced:       true,
				}
				if i < 2 {
					ps.RPCReachable = false
					ps.Synced = false
				} else {
					ps.AwaitingUpgrade = true
				}
				return ps
			})
		})

		control = NewPodControl(&mClient, podFilter)

		// Reconcile 2, should not update anything because 0 and 1 are still in progress.

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)

		// no further updates yet, should requeue.
		require.True(t, requeue)

		require.True(t, didFilter)

		require.Zero(t, mClient.CreateCount)

		// should not delete any more yet.
		require.Equal(t, 2, mClient.DeleteCount)

		// mock out that one of the pods completed the upgrade. should begin upgrading one more

		didFilter = false
		podFilter = mockPodFilter(func(_ context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
			require.Equal(t, namespace, crd.Namespace)
			require.Equal(t, "hub", crd.Name)
			didFilter = true
			return lo.Map(existing, func(pod *corev1.Pod, i int) cosmos.PodStatus {
				ps := cosmos.PodStatus{
					Pod:          pod,
					RPCReachable: true,
					Synced:       true,
				}
				if i == 1 {
					ps.RPCReachable = false
					ps.Synced = false
				}
				if i >= 2 {
					ps.AwaitingUpgrade = true
				}
				return ps
			})
		})

		control = NewPodControl(&mClient, podFilter)

		// Reconcile 3, should update 2 (only one) because 1 is still in progress, but 0 is done.

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)

		// only handled 1 updates, so should requeue.
		require.True(t, requeue)

		require.True(t, didFilter)

		require.Zero(t, mClient.CreateCount)

		// should delete one more
		require.Equal(t, 3, mClient.DeleteCount)

		existing[2].Spec.Containers[0].Image = "new-image"
		delete(existing[2].Labels, "app.kubernetes.io/revision")
		delete(existing[2].Annotations, "app.kubernetes.io/ordinal")
		rev2 := diff.Adapt(existing[2], 2).Revision()
		existing[2].Labels["app.kubernetes.io/revision"] = rev2
		existing[2].Annotations["app.kubernetes.io/ordinal"] = "2"
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		// mock out that both pods completed the upgrade. should begin upgrading the last 2

		didFilter = false
		podFilter = mockPodFilter(func(_ context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
			require.Equal(t, namespace, crd.Namespace)
			require.Equal(t, "hub", crd.Name)
			didFilter = true
			return lo.Map(existing, func(pod *corev1.Pod, i int) cosmos.PodStatus {
				ps := cosmos.PodStatus{
					Pod:          pod,
					RPCReachable: true,
					Synced:       true,
				}
				if i >= 3 {
					ps.AwaitingUpgrade = true
				}
				return ps
			})
		})

		control = NewPodControl(&mClient, podFilter)

		// Reconcile 4, should update 3 and 4 because the rest are done.

		requeue, err = control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)

		// all updates are now handled, no longer need requeue.
		require.False(t, requeue)

		require.True(t, didFilter)

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

		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		var didFilter bool
		podFilter := mockPodFilter(func(_ context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus {
			require.Equal(t, namespace, crd.Namespace)
			require.Equal(t, "hub", crd.Name)
			didFilter = true
			return lo.Map(existing, func(pod *corev1.Pod, i int) cosmos.PodStatus {
				return cosmos.PodStatus{
					Pod: pod,
					// pods are at or above upgrade height and not reachable
					AwaitingUpgrade: true,
					RPCReachable:    false,
					Synced:          false,
				}
			})
		})

		control := NewPodControl(&mClient, podFilter)

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 0, ready) // mockPodFilter returns no pods as synced, but all are at the upgrade height.
			return kube.ComputeRollout(maxUnavail, desired, ready)
		}

		// Trigger updates
		for _, pod := range existing {
			crd.Status.Height[pod.Name] = 100
		}

		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)

		// all updates are handled, so should not requeue
		require.False(t, requeue)

		require.True(t, didFilter)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, 5, mClient.DeleteCount)
	})
}
