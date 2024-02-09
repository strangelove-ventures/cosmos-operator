package fullnode

import (
	"context"
	"errors"
	"github.com/go-resty/resty/v2"
	"testing"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
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
		nodeKey   = `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"HBX8VFQ4OdWfOwIOR7jj0af8mVHik5iGW9o1xnn4vRltk1HmwQS2LLGrMPVS2LIUO9BUqmZ1Pjt+qM8x0ibHxQ=="}}`
	)

	t.Run("happy path - private addresses", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 2
		res, err := BuildNodeKeySecrets(nil, &crd)
		require.NoError(t, err)
		secret := res[0].Object()
		secret.Data[nodeKeyFile] = []byte(nodeKey)

		var (
			getCount int
			objKeys  []client.ObjectKey
		)
		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			objKeys = append(objKeys, key)
			getCount++
			switch ref := obj.(type) {
			case *corev1.Secret:
				*ref = *secret
			case *corev1.Service:
				*ref = corev1.Service{}
			}
			return nil
		})

		collector := NewPeerCollector(getter)
		peers, err := collector.Collect(ctx, &crd)
		require.NoError(t, err)
		require.Len(t, peers, 2)

		require.Equal(t, 4, getCount) // 2 secrets + 2 services

		wantKeys := []client.ObjectKey{
			{Name: "dydx-node-key-0", Namespace: namespace},
			{Name: "dydx-p2p-0", Namespace: namespace},
			{Name: "dydx-node-key-1", Namespace: namespace},
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
		crd.Spec.Replicas = 4
		res, err := BuildNodeKeySecrets(nil, &crd)
		require.NoError(t, err)
		secret := res[0].Object()
		secret.Data[nodeKeyFile] = []byte(nodeKey)

		c := resty.New()
		resp, err := c.R().
			Get("https://ipv4.icanhazip.com")

		externalIP := resp.String()

		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			switch ref := obj.(type) {
			case *corev1.Secret:
				*ref = *secret
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
				case "dydx-p2p-3":
					svc.Spec.Type = corev1.ServiceTypeNodePort
					svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{Port: 26656, Name: "p2p", NodePort: 30000})
				}
				*ref = svc
			}
			return nil
		})

		collector := NewPeerCollector(getter)
		peers, err := collector.Collect(ctx, &crd)
		require.NoError(t, err)
		require.Len(t, peers, 4)

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

		got = peers[client.ObjectKey{Name: "dydx-3", Namespace: namespace}]
		require.Equal(t, externalIP+":30000", got.ExternalAddress)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@"+externalIP+":30000", got.ExternalPeer())

		require.True(t, peers.HasIncompleteExternalAddress())
		want := []string{
			"1e23ce0b20ae2377925537cc71d1529d723bb892@0.0.0.0:26656",
			"1e23ce0b20ae2377925537cc71d1529d723bb892@1.2.3.4:26656",
			"1e23ce0b20ae2377925537cc71d1529d723bb892@host.example.com:26656",
			"1e23ce0b20ae2377925537cc71d1529d723bb892@" + externalIP + ":30000",
		}
		require.ElementsMatch(t, want, peers.AllExternal())
	})

	t.Run("zero replicas", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Spec.Replicas = 0

		collector := NewPeerCollector(panicGetter)
		peers, err := collector.Collect(ctx, &crd)
		require.NoError(t, err)
		require.Len(t, peers, 0)
	})

	t.Run("get error", func(t *testing.T) {
		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			return errors.New("boom")
		})

		collector := NewPeerCollector(getter)
		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Spec.Replicas = 1
		_, err := collector.Collect(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "get secret dydx-node-key-0: boom")
		require.True(t, err.IsTransient())
	})

	t.Run("invalid node key", func(t *testing.T) {
		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			switch ref := obj.(type) {
			case *corev1.Secret:
				var secret corev1.Secret
				secret.Data = map[string][]byte{nodeKeyFile: []byte("invalid")}
				*ref = secret
			case *corev1.Service:
				panic("should not be called")
			}
			return nil
		})

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Spec.Replicas = 1
		collector := NewPeerCollector(getter)
		_, err := collector.Collect(ctx, &crd)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid character")
		require.False(t, err.IsTransient())
	})
}
