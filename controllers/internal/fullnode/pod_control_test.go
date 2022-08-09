package fullnode

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var nopLogger = logr.Discard()

func TestPodControl_Reconcile(t *testing.T) {
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
		var mClient mockClient
		mClient.PodList = corev1.PodList{
			Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
			},
		}

		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Namespace = namespace
		crd.Name = "hub"

		control := NewPodControl(nopLogger, &mClient)
		control.diffFactory = func(ordinalAnnotationKey string, current, want []*corev1.Pod) differ {
			require.Equal(t, "cosmosfullnode.cosmos.strange.love/ordinal", ordinalAnnotationKey)
			require.Len(t, current, 1)
			require.Equal(t, "pod-1", mClient.PodList.Items[0].Name)
			require.Len(t, want, 3)
			return mockDiffer{}
		}
		err := control.Reconcile(ctx, &crd)
		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 3)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, namespace, listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, "cosmosfullnode.cosmos.strange.love/chain-name=hub", listOpt.LabelSelector.String())
		require.Equal(t, ".metadata.controller=hub", listOpt.FieldSelector.String())
	})

	t.Run("scale phase", func(t *testing.T) {
		var (
			mDiff = mockDiffer{
				StubCreates: buildPods(3),
				StubDeletes: buildPods(2),
				StubUpdates: buildPods(10),
			}
			mClient mockClient
			crd     = defaultCRD()
			control = NewPodControl(nopLogger, &mClient)
		)
		crd.Namespace = namespace
		control.diffFactory = func(ordinalAnnotationKey string, current, want []*corev1.Pod) differ {
			return mDiff
		}
		err := control.Reconcile(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "scaling in progress")
		require.True(t, err.IsTransient())

		require.Equal(t, 3, mClient.CreateCount)
		require.Equal(t, 2, mClient.DeleteCount)

		require.NotEmpty(t, mClient.LastCreatedPod.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreatedPod.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreatedPod.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreatedPod.OwnerReferences[0].Controller)
	})

	t.Run("rollout phase", func(t *testing.T) {
		var mClient mockClient
		mClient.PodList = corev1.PodList{
			Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
			},
		}

		var (
			mDiff = mockDiffer{
				StubUpdates: buildPods(10),
			}
			crd     = defaultCRD()
			control = NewPodControl(nopLogger, &mClient)
		)

		crd.Namespace = namespace
		crd.Spec.Replicas = 10
		control.diffFactory = func(ordinalAnnotationKey string, current, want []*corev1.Pod) differ {
			return mDiff
		}

		const stubRollout = 5
		control.computeRollout = func(maxUnavail *intstr.IntOrString, desired, ready int) int {
			require.EqualValues(t, crd.Spec.Replicas, desired)
			return stubRollout
		}

		err := control.Reconcile(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "rollout in progress")
		require.True(t, err.IsTransient())

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, stubRollout, mClient.DeleteCount)
	})
}

type mockDiffer struct {
	StubCreates, StubUpdates, StubDeletes []*corev1.Pod
}

func (m mockDiffer) Creates() []*corev1.Pod {
	return m.StubCreates
}

func (m mockDiffer) Updates() []*corev1.Pod {
	return m.StubUpdates
}

func (m mockDiffer) Deletes() []*corev1.Pod {
	return m.StubDeletes
}

type mockClient struct {
	PodList     corev1.PodList
	GotListOpts []client.ListOption

	CreateCount    int
	LastCreatedPod *corev1.Pod
	DeleteCount    int
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	panic("implement me")
}

func (m *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.GotListOpts = opts
	ref := list.(*corev1.PodList)
	*ref = m.PodList
	return nil
}

func (m *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.LastCreatedPod = obj.(*corev1.Pod)
	m.CreateCount++
	return nil
}

func (m *mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	m.DeleteCount++
	return nil
}

func (m *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	panic("implement me")
}

func (m *mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	panic("implement me")
}

func (m *mockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

func (m *mockClient) Scheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := cosmosv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return scheme
}
