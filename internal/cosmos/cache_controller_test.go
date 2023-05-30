package cosmos

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type mockCollector struct {
	Called         int64
	GotPods        []corev1.Pod
	StubCollection StatusCollection
}

func (m *mockCollector) Collect(ctx context.Context, pods []corev1.Pod) StatusCollection {
	atomic.AddInt64(&m.Called, 1)
	if ctx == nil {
		panic("nil context")
	}
	m.GotPods = pods
	return m.StubCollection
}

type mockReader struct {
	sync.Mutex
	GetErr error

	ListPods []corev1.Pod
	ListOpts []client.ListOption
}

func (m *mockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	m.Lock()
	defer m.Unlock()
	if ctx == nil {
		panic("nil context")
	}
	var crd cosmosv1.CosmosFullNode
	crd.Name = key.Name
	crd.Namespace = key.Namespace
	*obj.(*cosmosv1.CosmosFullNode) = crd
	return m.GetErr
}

func (m *mockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.Lock()
	defer m.Unlock()
	if ctx == nil {
		panic("nil context")
	}
	m.ListOpts = opts
	list.(*corev1.PodList).Items = m.ListPods
	return nil
}

func TestCacheController_Reconcile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const (
		namespace = "strangelove"
		name      = "nolus"
	)

	validStatusColl := StatusCollection{
		{pod: new(corev1.Pod)},
		{pod: new(corev1.Pod)},
		{pod: new(corev1.Pod)},
	}

	t.Run("crd created or updated", func(t *testing.T) {
		defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

		pods := make([]corev1.Pod, 2)
		var reader mockReader
		reader.ListPods = pods

		var collector mockCollector
		collector.StubCollection = validStatusColl

		controller := NewCacheController(&collector, &reader, nil)

		var req reconcile.Request
		req.Name = name
		req.Namespace = namespace

		// Ensures we don't cache more than once per request
		for i := 0; i < 3; i++ {
			res, err := controller.Reconcile(ctx, req)
			require.Equal(t, reconcile.Result{}, res)
			require.NoError(t, err)
		}

		key := client.ObjectKey{Name: name, Namespace: namespace}
		require.Eventually(t, func() bool {
			got := controller.Collect(ctx, key)
			return len(got) == 3
		}, time.Second, time.Millisecond)

		require.Equal(t, collector.StubCollection, controller.Collect(ctx, key))
		require.Equal(t, pods, collector.GotPods)

		opts := reader.ListOpts
		require.Len(t, opts, 2)
		var listOpt client.ListOptions
		for _, opt := range opts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, namespace, listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=nolus", listOpt.FieldSelector.String())

		require.NoError(t, controller.Close())

		require.Equal(t, int64(1), collector.Called)
	})

	t.Run("crd deleted", func(t *testing.T) {
		defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

		pods := make([]corev1.Pod, 1)
		reader := &mockReader{}
		reader.ListPods = pods

		var collector mockCollector
		collector.StubCollection = validStatusColl[:1]

		controller := NewCacheController(&collector, reader, nil)

		var req reconcile.Request
		req.Name = name
		req.Namespace = namespace
		_, err := controller.Reconcile(ctx, req)
		require.NoError(t, err)

		key := client.ObjectKey{Name: name, Namespace: namespace}
		require.Eventually(t, func() bool {
			return len(controller.Collect(ctx, key)) > 0
		}, time.Second, time.Millisecond)

		reader.GetErr = apierrors.NewNotFound(schema.GroupResource{}, name)

		_, err = controller.Reconcile(ctx, req)
		require.NoError(t, err)

		reader.ListPods = nil
		require.Empty(t, controller.Collect(ctx, key))

		require.NoError(t, controller.Close())
	})

	t.Run("zero state", func(t *testing.T) {
		defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

		var reader mockReader
		var collector mockCollector
		collector.StubCollection = make(StatusCollection, 1)

		controller := NewCacheController(&collector, &reader, nil)
		key := client.ObjectKey{Name: name, Namespace: namespace}
		require.Empty(t, controller.Collect(ctx, key))
		require.Empty(t, controller.SyncedPods(ctx, key))
	})
}

func TestCacheController_SyncedPods(t *testing.T) {
	t.Parallel()

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx := context.Background()
	const (
		namespace = "default"
		name      = "axelar"
	)

	reader := new(mockReader)
	reader.ListPods = make([]corev1.Pod, 2)

	var catchingUp CometStatus
	catchingUp.Result.SyncInfo.CatchingUp = true

	var collector mockCollector
	collector.StubCollection = StatusCollection{
		{pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "1"}}},
		{pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "2"}}, status: catchingUp},
		{pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "should not see me"}}, status: catchingUp},
	}

	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{UID: "1"}},
	}
	reader.ListPods = pods

	controller := NewCacheController(&collector, reader, nil)

	var req reconcile.Request
	req.Name = name
	req.Namespace = namespace
	_, err := controller.Reconcile(ctx, req)
	require.NoError(t, err)

	key := client.ObjectKey{Name: name, Namespace: namespace}
	require.Eventually(t, func() bool {
		return len(controller.Collect(ctx, key)) > 0
	}, time.Second, time.Millisecond)

	readyStatus := corev1.PodStatus{Conditions: []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Second))}},
	}
	pods = []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{UID: "1"}, Status: readyStatus},
		{ObjectMeta: metav1.ObjectMeta{UID: "2"}},
		{ObjectMeta: metav1.ObjectMeta{UID: "new"}},
	}

	reader.Lock()
	reader.ListPods = pods
	reader.Unlock()

	gotColl := controller.Collect(ctx, key)
	uids := lo.Map(gotColl, func(item StatusItem, _ int) string { return string(item.pod.UID) })
	require.Equal(t, []string{"1", "2", "new"}, uids)

	_, err = gotColl[2].Status()
	require.Error(t, err)
	require.EqualError(t, err, "missing status")

	gotPods := controller.SyncedPods(ctx, key)
	require.Len(t, gotPods, 1)
	require.Equal(t, pods[0], *gotPods[0])

	require.NoError(t, controller.Close())

	opts := reader.ListOpts
	require.Len(t, opts, 2)
	var listOpt client.ListOptions
	for _, opt := range opts {
		opt.ApplyToList(&listOpt)
	}
	require.Equal(t, namespace, listOpt.Namespace)
	require.Zero(t, listOpt.Limit)
	require.Equal(t, ".metadata.controller=axelar", listOpt.FieldSelector.String())
}
