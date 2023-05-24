package cosmos

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type mockCollector struct {
	Called         int64
	WaitCh         chan struct{}
	GotPods        []corev1.Pod
	StubCollection StatusCollection
}

func (m *mockCollector) Collect(ctx context.Context, pods []corev1.Pod) StatusCollection {
	if m.WaitCh == nil {
		panic("nil wait channel")
	}
	defer func() { m.WaitCh <- struct{}{} }()
	atomic.AddInt64(&m.Called, 1)
	if ctx == nil {
		panic("nil context")
	}
	m.GotPods = pods
	return m.StubCollection
}

type mockReader struct {
	ListPods []corev1.Pod
	ListOpts []client.ListOption
}

func (m *mockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if ctx == nil {
		panic("nil context")
	}
	var crd cosmosv1.CosmosFullNode
	crd.Name = key.Name
	crd.Namespace = key.Namespace
	*obj.(*cosmosv1.CosmosFullNode) = crd
	return nil
}

func (m *mockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.ListOpts = opts
	list.(*corev1.PodList).Items = m.ListPods
	return nil
}

func TestCacheController_Reconcile(t *testing.T) {
	mockRecorder := struct {
		record.EventRecorder
	}{}

	ctx := context.Background()
	const (
		namespace = "strangelove"
		name      = "nolus"
	)

	t.Run("crd created or updated", func(t *testing.T) {
		defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

		pods := make([]corev1.Pod, 2)
		var reader mockReader
		reader.ListPods = pods

		var collector mockCollector
		collector.WaitCh = make(chan struct{}, 1)
		collector.StubCollection = make(StatusCollection, 3)

		controller := NewCacheController(&collector, &reader, mockRecorder)

		var req reconcile.Request
		req.Name = name
		req.Namespace = namespace
		res, err := controller.Reconcile(ctx, req)

		require.Equal(t, reconcile.Result{}, res)
		require.NoError(t, err)

		<-collector.WaitCh
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

		require.Eventually(t, func() bool {
			gotColl := controller.Collect(client.ObjectKey{Name: name, Namespace: namespace})
			return len(gotColl) == 3
		}, time.Second, time.Millisecond)

		require.NoError(t, controller.Close())

		require.Equal(t, int64(1), collector.Called)
	})

	t.Run("crd deleted", func(t *testing.T) {
		t.Fail()
	})
}
