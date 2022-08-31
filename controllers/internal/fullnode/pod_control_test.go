package fullnode

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var nopLogger = logr.Discard()

func TestPodControl_Reconcile(t *testing.T) {
	t.Parallel()

	type (
		mockPodClient = mockClient[*corev1.Pod]
		mockPodDiffer = mockDiffer[*corev1.Pod]
	)
	ctx := context.Background()
	const namespace = "testns"

	buildPods := func(n int) []*corev1.Pod {
		return lo.Map(lo.Range(n), func(i int, _ int) *corev1.Pod {
			var pod corev1.Pod
			pod.Name = fmt.Sprintf("pod-%d", i)
			pod.Namespace = namespace
			// Mark pod as Ready.
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			return &pod
		})
	}

	t.Run("no changes", func(t *testing.T) {
		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
			},
		}

		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Namespace = namespace
		crd.Name = "hub"

		control := NewPodControl(&mClient)
		control.diffFactory = func(ordinalAnnotationKey, revisionLabelKey string, current, want []*corev1.Pod) podDiffer {
			require.Equal(t, "app.kubernetes.io/ordinal", ordinalAnnotationKey)
			require.Equal(t, "app.kubernetes.io/revision", revisionLabelKey)
			require.Len(t, current, 1)
			require.Equal(t, "pod-1", current[0].Name)
			require.Len(t, want, 3)
			return mockPodDiffer{}
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
			mDiff = mockPodDiffer{
				StubCreates: buildPods(3),
				StubDeletes: buildPods(2),
				StubUpdates: buildPods(10),
			}
			mClient mockPodClient
			crd     = defaultCRD()
			control = NewPodControl(&mClient)
		)
		crd.Namespace = namespace
		control.diffFactory = func(_, _ string, current, want []*corev1.Pod) podDiffer {
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

	t.Run("rollout phase", func(t *testing.T) {
		var mClient mockPodClient
		mClient.ObjectList = corev1.PodList{
			Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
			},
		}

		var (
			mDiff = mockPodDiffer{
				StubUpdates: buildPods(10),
			}
			crd     = defaultCRD()
			control = NewPodControl(&mClient)
		)

		crd.Namespace = namespace
		crd.Spec.Replicas = 10
		control.diffFactory = func(_, _ string, _, _ []*corev1.Pod) podDiffer {
			return mDiff
		}

		const stubRollout = 5
		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			return stubRollout
		}

		requeue, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)
		require.True(t, requeue)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, stubRollout, mClient.DeleteCount)
	})
}
