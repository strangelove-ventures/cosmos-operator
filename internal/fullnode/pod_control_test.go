package fullnode

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type mockPodCollection struct {
	pods   []*corev1.Pod
	synced []*corev1.Pod
}

func (m mockPodCollection) Pods() []*corev1.Pod {
	return m.pods
}

func (m mockPodCollection) SyncedPods() []*corev1.Pod {
	return m.synced
}

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

		collection := mockPodCollection{pods: []*corev1.Pod{existing}}
		control := NewPodControl(&mClient)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, collection, nil)
		require.NoError(t, err)
		require.False(t, requeue)
	})

	t.Run("scale phase", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "hub"
		crd.Namespace = namespace
		crd.Spec.Replicas = 3

		var mClient mockPodClient
		collection := mockPodCollection{pods: []*corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "hub-98"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "hub-99"}},
		}}

		control := NewPodControl(&mClient)
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, collection, nil)
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

		collection := mockPodCollection{
			pods:   existing,
			synced: existing[2:], // Make only 3 ready
		}

		control := NewPodControl(&mClient)
		const stubRollout = 5
		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			require.Equal(t, 3, ready) // mock only returns 4 pod as in sync
			return stubRollout
		}

		// Trigger updates
		crd.Spec.PodTemplate.Image = "new-image"
		requeue, err := control.Reconcile(ctx, nopReporter, &crd, collection, nil)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, stubRollout, mClient.DeleteCount)
	})
}
