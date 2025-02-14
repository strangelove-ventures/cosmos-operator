package fullnode

import (
	"context"
	"errors"
	"fmt"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockGetter func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error

func (fn mockGetter) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if ctx == nil {
		panic("nil context")
	}
	if len(opts) > 0 {
		panic("unexpected opts")
	}
	return fn(ctx, key, obj, opts...)
}

var panicGetter = mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	panic("should not be called")
})

func TestPeerCollector_Collect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const (
		namespace = "strangelove"
	)

	t.Run("happy path - private addresses", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 2

		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		var (
			getCount int
			objKeys  []client.ObjectKey
		)
		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			objKeys = append(objKeys, key)
			getCount++
			switch ref := obj.(type) {
			case *corev1.Service:
				*ref = corev1.Service{}
			}
			return nil
		})

		collector := NewPeerCollector(getter)
		peers, err := collector.Collect(ctx, &crd, nodeKeys)
		require.NoError(t, err)
		require.Len(t, peers, 2)

		require.Equal(t, 2, getCount) // 2 services

		wantKeys := []client.ObjectKey{
			{Name: "dydx-p2p-0", Namespace: namespace},
			{Name: "dydx-p2p-1", Namespace: namespace},
		}
		require.Equal(t, wantKeys, objKeys)

		got := peers[client.ObjectKey{Name: "dydx-0", Namespace: namespace}]
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892", got.NodeID)
		require.Equal(t, "dydx-p2p-0.strangelove.svc.cluster.local:26656", got.PrivateAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@dydx-p2p-0.strangelove.svc.cluster.local:26656", got.PrivatePeer())
		require.Empty(t, got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@0.0.0.0:26656", got.ExternalPeer())

		got = peers[client.ObjectKey{Name: "dydx-1", Namespace: namespace}]
		require.NotEmpty(t, got.NodeID)
		require.Equal(t, "dydx-p2p-1.strangelove.svc.cluster.local:26656", got.PrivateAddress)
		require.Empty(t, got.ExternalAddress)

		require.False(t, peers.HasIncompleteExternalAddress())
	})

	t.Run("happy path - external addresses", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 3

		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			switch ref := obj.(type) {
			case *corev1.Service:
				var svc corev1.Service
				switch key.Name {
				case "dydx-p2p-0":
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				case "dydx-p2p-1":
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
					svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}}
				case "dydx-p2p-2":
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
					svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: "host.example.com"}}
				}
				*ref = svc
			}
			return nil
		})

		collector := NewPeerCollector(getter)
		peers, err := collector.Collect(ctx, &crd, nodeKeys)
		require.NoError(t, err)
		require.Len(t, peers, 3)

		got := peers[client.ObjectKey{Name: "dydx-0", Namespace: namespace}]
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892", got.NodeID)
		require.Empty(t, got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@0.0.0.0:26656", got.ExternalPeer())

		got = peers[client.ObjectKey{Name: "dydx-1", Namespace: namespace}]
		require.Equal(t, "1.2.3.4:26656", got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@1.2.3.4:26656", got.ExternalPeer())

		got = peers[client.ObjectKey{Name: "dydx-2", Namespace: namespace}]
		require.Equal(t, "host.example.com:26656", got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@host.example.com:26656", got.ExternalPeer())

		require.True(t, peers.HasIncompleteExternalAddress())
		want := []string{"1e23ce0b20ae2377925537cc71d1529d723bb892@0.0.0.0:26656",
			"1e23ce0b20ae2377925537cc71d1529d723bb892@1.2.3.4:26656",
			"1e23ce0b20ae2377925537cc71d1529d723bb892@host.example.com:26656"}
		require.ElementsMatch(t, want, peers.AllExternal())
	})

	t.Run("happy path  with non 0 starting ordinal- private addresses", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 2
		crd.Spec.Ordinals.Start = 2

		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		var (
			getCount int
			objKeys  []client.ObjectKey
		)
		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			objKeys = append(objKeys, key)
			getCount++
			switch ref := obj.(type) {
			case *corev1.Service:
				*ref = corev1.Service{}
			}
			return nil
		})

		collector := NewPeerCollector(getter)
		peers, err := collector.Collect(ctx, &crd, nodeKeys)
		require.NoError(t, err)
		require.Len(t, peers, 2)

		require.Equal(t, 2, getCount) // 2 services

		wantKeys := []client.ObjectKey{
			{Name: fmt.Sprintf("dydx-p2p-%d", crd.Spec.Ordinals.Start), Namespace: namespace},
			{Name: fmt.Sprintf("dydx-p2p-%d", crd.Spec.Ordinals.Start+1), Namespace: namespace},
		}
		require.Equal(t, wantKeys, objKeys)

		got := peers[client.ObjectKey{Name: fmt.Sprintf("dydx-%d", crd.Spec.Ordinals.Start), Namespace: namespace}]
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892", got.NodeID)
		require.Equal(t, fmt.Sprintf("dydx-p2p-%d.strangelove.svc.cluster.local:26656", crd.Spec.Ordinals.Start), got.PrivateAddress)
		require.Equal(t, fmt.Sprintf("1e23ce0b20ae2377925537cc71d1529d723bb892@dydx-p2p-%d.strangelove.svc.cluster.local:26656", crd.Spec.Ordinals.Start), got.PrivatePeer())
		require.Empty(t, got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@0.0.0.0:26656", got.ExternalPeer())

		got = peers[client.ObjectKey{Name: fmt.Sprintf("dydx-%d", crd.Spec.Ordinals.Start+1), Namespace: namespace}]
		require.NotEmpty(t, got.NodeID)
		require.Equal(t, fmt.Sprintf("dydx-p2p-%d.strangelove.svc.cluster.local:26656", crd.Spec.Ordinals.Start+1), got.PrivateAddress)
		require.Empty(t, got.ExternalAddress)

		require.False(t, peers.HasIncompleteExternalAddress())
	})

	t.Run("happy path with non 0 starting ordinal - external addresses", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 3
		crd.Spec.Ordinals.Start = 0

		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			switch ref := obj.(type) {
			case *corev1.Service:
				var svc corev1.Service
				switch key.Name {
				case fmt.Sprintf("dydx-p2p-%d", crd.Spec.Ordinals.Start):
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				case fmt.Sprintf("dydx-p2p-%d", crd.Spec.Ordinals.Start+1):
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
					svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}}
				case fmt.Sprintf("dydx-p2p-%d", crd.Spec.Ordinals.Start+2):
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
					svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: "host.example.com"}}
				}
				*ref = svc
			}
			return nil
		})

		collector := NewPeerCollector(getter)
		peers, err := collector.Collect(ctx, &crd, nodeKeys)
		require.NoError(t, err)
		require.Len(t, peers, 3)

		got := peers[client.ObjectKey{Name: fmt.Sprintf("dydx-%d", crd.Spec.Ordinals.Start), Namespace: namespace}]
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892", got.NodeID)
		require.Empty(t, got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@0.0.0.0:26656", got.ExternalPeer())

		got = peers[client.ObjectKey{Name: fmt.Sprintf("dydx-%d", crd.Spec.Ordinals.Start+1), Namespace: namespace}]
		require.Equal(t, "1.2.3.4:26656", got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@1.2.3.4:26656", got.ExternalPeer())

		got = peers[client.ObjectKey{Name: fmt.Sprintf("dydx-%d", crd.Spec.Ordinals.Start+2), Namespace: namespace}]
		require.Equal(t, "host.example.com:26656", got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@host.example.com:26656", got.ExternalPeer())

		require.True(t, peers.HasIncompleteExternalAddress())
		want := []string{
			"1e23ce0b20ae2377925537cc71d1529d723bb892@0.0.0.0:26656",
			"1e23ce0b20ae2377925537cc71d1529d723bb892@1.2.3.4:26656",
			"1e23ce0b20ae2377925537cc71d1529d723bb892@host.example.com:26656",
		}
		require.ElementsMatch(t, want, peers.AllExternal())
	})

	t.Run("zero replicas", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Spec.Replicas = 0

		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		collector := NewPeerCollector(panicGetter)
		peers, err := collector.Collect(ctx, &crd, nodeKeys)
		require.NoError(t, err)
		require.Len(t, peers, 0)
	})

	t.Run("get error", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Spec.Replicas = 1

		nodeKeys, nErr := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, nErr)

		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			return errors.New("boom")
		})

		collector := NewPeerCollector(getter)
		_, err := collector.Collect(ctx, &crd, nodeKeys)

		require.Error(t, err)
		require.EqualError(t, err, "get server dydx-p2p-0: boom")
		require.True(t, err.IsTransient())
	})
}
