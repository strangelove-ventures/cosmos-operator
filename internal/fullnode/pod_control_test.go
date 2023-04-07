package fullnode

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockPodFilter func(ctx context.Context, log kube.Logger, candidates []*corev1.Pod) []*corev1.Pod

func (fn mockPodFilter) SyncedPods(ctx context.Context, log kube.Logger, candidates []*corev1.Pod) []*corev1.Pod {
	if ctx == nil {
		panic("nil context")
	}
	return fn(ctx, log, candidates)
}

var panicPodFilter = mockPodFilter(func(ctx context.Context, log kube.Logger, candidates []*corev1.Pod) []*corev1.Pod {
	panic("SyncedPods should not be called")
})

func TestPodControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockPodClient = mockClient[*corev1.Pod]

	ctx := context.Background()
	const namespace = "testns"

	t.Run("no changes", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1

		pods, err := BuildPods(&crd)
		require.NoError(t, err)
		existing := diff.New(nil, pods).Creates()[0]

		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: []corev1.Pod{*existing},
		}

		control := NewPodControl(&mClient, panicPodFilter)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd)
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
		var (
			mClient mockPodClient
			crd     = defaultCRD()
			control = NewPodControl(&mClient, panicPodFilter)
		)
		crd.Namespace = namespace
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

	t.Run("rollout phase", func(t *testing.T) {
		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}},
			},
		}

		var (
			crd       = defaultCRD()
			podFilter = mockPodFilter(func(_ context.Context, _ kube.Logger, candidates []*corev1.Pod) []*corev1.Pod {
				require.Equal(t, 2, len(candidates))
				require.Equal(t, "pod-1", candidates[0].Name)
				require.Equal(t, "pod-2", candidates[1].Name)
				return candidates[:1]
			})
			control = NewPodControl(&mClient, podFilter)
		)

		crd.Namespace = namespace
		crd.Spec.Replicas = 10

		const stubRollout = 5
		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 1, ready) // mockPodFilter only returns 1 candidate as ready
			return stubRollout
		}

		requeue, err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, stubRollout, mClient.DeleteCount)
	})
}
