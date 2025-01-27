package fullnode

import (
	"context"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNodeKeyCollector_Collect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const (
		namespace = "strangelove"
		nodeKey1  = `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"HBX8VFQ4OdWfOwIOR7jj0af8mVHik5iGW9o1xnn4vRltk1HmwQS2LLGrMPVS2LIUO9BUqmZ1Pjt+qM8x0ibHxQ=="}}`
		nodeKey2  = `{"priv_key": {"type": "tendermint/PrivKeyEd25519", "value": "1JJ0C2TqVfbwgrrCKQiFr1wpWWwOeiJXl4CLcuk2Uot9gnf9hEHmfITWXCQRGvtdXU6uL1v6Ri00i4aEm00DLw=="}}`
	)

	type mockConfigClient = mockClient[*corev1.ConfigMap]

	t.Run("happy path - non-existant node keys in old config maps", func(t *testing.T) {

		var mClient mockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{}}

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 2

		collector := NewNodeKeyCollector(&mClient)

		nodeKeys, err := collector.Collect(ctx, &crd)

		require.NoError(t, err)

		require.Len(t, nodeKeys, 2)

	})

	t.Run("happy path - existing node keys in old config maps", func(t *testing.T) {

		var mClient mockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "dydx-0", Namespace: namespace},
				Data:       map[string]string{nodeKeyFile: nodeKey1},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "dydx-1", Namespace: namespace},
				Data:       map[string]string{nodeKeyFile: nodeKey2},
			},
		}}

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 2

		collector := NewNodeKeyCollector(&mClient)

		nodeKeys, err := collector.Collect(ctx, &crd)

		require.NoError(t, err)

		require.Len(t, nodeKeys, 2)

		require.Equal(t, nodeKey1, string(nodeKeys[client.ObjectKey{Name: "dydx-0", Namespace: namespace}].MarshalledNodeKey))
		require.Equal(t, nodeKey2, string(nodeKeys[client.ObjectKey{Name: "dydx-1", Namespace: namespace}].MarshalledNodeKey))
	})
}
