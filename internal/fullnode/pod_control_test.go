package fullnode

import (
	"context"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockPodFilter func(ctx context.Context, crd *cosmosv1.CosmosFullNode) []*corev1.Pod

func (fn mockPodFilter) ReadyPods(ctx context.Context, crd *cosmosv1.CosmosFullNode) []*corev1.Pod {
	if ctx == nil {
		panic("nil context")
	}
	return fn(ctx, crd)
}

var panicPodFilter = mockPodFilter(func(context.Context, *cosmosv1.CosmosFullNode) []*corev1.Pod {
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

		pods, err := BuildPods(&crd, nil)
		require.NoError(t, err)
		existing := diff.New(nil, pods).Creates()

		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: valueSlice(existing),
		}

		var didFilter bool
		podFilter := mockPodFilter(func(_ context.Context, crd *cosmosv1.CosmosFullNode) []*corev1.Pod {
			require.Equal(t, namespace, crd.Namespace)
			require.Equal(t, "hub", crd.Name)
			didFilter = true
			return existing[:1]
		})

		control := NewPodControl(&mClient, podFilter)
		const stubRollout = 5

		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 1, ready) // mockPodFilter only returns 1 candidate as ready
			return stubRollout
		}

		// Trigger updates
		crd.Spec.PodTemplate.Image = "new-image"
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)
		require.True(t, requeue)

		require.True(t, didFilter)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, 1, mClient.DeleteCount)
	})
}
