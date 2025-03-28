package fullnode

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

	t.Run("happy path - non-existent node keys in old config maps", func(t *testing.T) {
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

		require.Equal(t, nodeKey1, string(nodeKeys[client.ObjectKey{Name: "dydx-0", Namespace: namespace}].MarshaledNodeKey))
		require.Equal(t, nodeKey2, string(nodeKeys[client.ObjectKey{Name: "dydx-1", Namespace: namespace}].MarshaledNodeKey))
	})
}

type nodeKeyMockConfigClient = mockClient[*corev1.ConfigMap]

var defaultMockNodeKeyData = `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"HBX8VFQ4OdWfOwIOR7jj0af8mVHik5iGW9o1xnn4vRltk1HmwQS2LLGrMPVS2LIUO9BUqmZ1Pjt+qM8x0ibHxQ=="}}`

func getMockNodeKeysForCRD(crd cosmosv1.CosmosFullNode, mockNodeKeyData string) (NodeKeys, error) {
	var nodeKey = mockNodeKeyData

	if nodeKey == "" {
		nodeKey = defaultMockNodeKeyData
	}

	configMapItems := []corev1.ConfigMap{}

	for i := crd.Spec.Ordinals.Start; i < crd.Spec.Ordinals.Start+crd.Spec.Replicas; i++ {
		configMapItems = append(configMapItems, corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-%d", crd.Name, i), Namespace: crd.Namespace},
			Data:       map[string]string{nodeKeyFile: nodeKey},
		})
	}

	var mClient nodeKeyMockConfigClient
	mClient.ObjectList = corev1.ConfigMapList{Items: configMapItems}

	collector := NewNodeKeyCollector(&mClient)
	ctx := context.Background()

	return collector.Collect(ctx, &crd)
}

func TestNodeKeyCollector_InvalidJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const (
		namespace      = "strangelove"
		invalidNodeKey = `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value": INVALID JSON}}`
	)

	var mClient nodeKeyMockConfigClient
	mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "dydx-0", Namespace: namespace},
			Data:       map[string]string{nodeKeyFile: invalidNodeKey},
		},
	}}

	var crd cosmosv1.CosmosFullNode
	crd.Name = "dydx"
	crd.Namespace = namespace
	crd.Spec.Replicas = 1

	collector := NewNodeKeyCollector(&mClient)

	_, err := collector.Collect(ctx, &crd)

	require.Error(t, err)
}

func TestNodeKeyCollector_NonZeroOrdinals(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const (
		namespace = "strangelove"
		nodeKey1  = `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"HBX8VFQ4OdWfOwIOR7jj0af8mVHik5iGW9o1xnn4vRltk1HmwQS2LLGrMPVS2LIUO9BUqmZ1Pjt+qM8x0ibHxQ=="}}`
	)

	var mClient nodeKeyMockConfigClient
	mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "dydx-5", Namespace: namespace},
			Data:       map[string]string{nodeKeyFile: nodeKey1},
		},
	}}

	var crd cosmosv1.CosmosFullNode
	crd.Name = "dydx"
	crd.Namespace = namespace
	crd.Spec.Replicas = 2
	crd.Spec.Ordinals.Start = 5

	collector := NewNodeKeyCollector(&mClient)

	nodeKeys, err := collector.Collect(ctx, &crd)

	require.NoError(t, err)
	require.Len(t, nodeKeys, 2)
	require.Contains(t, nodeKeys, client.ObjectKey{Name: "dydx-5", Namespace: namespace})
	require.Contains(t, nodeKeys, client.ObjectKey{Name: "dydx-6", Namespace: namespace})
}

func TestNodeKey_ID(t *testing.T) {
	t.Parallel()

	// Create a known node key with fixed private key for testing
	privateKey := ed25519.PrivateKey([]byte("test-private-key-for-deterministic-id-generation"))
	nodeKey := NodeKey{
		PrivKey: NodeKeyPrivKey{
			Type:  "tendermint/PrivKeyEd25519",
			Value: privateKey,
		},
	}

	// Calculate expected ID manually
	pub := privateKey.Public()
	hash := sha256.Sum256(pub.(ed25519.PublicKey))
	expectedID := hex.EncodeToString(hash[:20])

	actualID := nodeKey.ID()

	require.Equal(t, expectedID, actualID)
}

func TestNodeKeyCollector_DecreaseReplicas(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const namespace = "strangelove"

	var mClient nodeKeyMockConfigClient
	mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "dydx-0", Namespace: namespace},
			Data:       map[string]string{nodeKeyFile: `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"key1"}}`},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "dydx-1", Namespace: namespace},
			Data:       map[string]string{nodeKeyFile: `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"key2"}}`},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "dydx-2", Namespace: namespace},
			Data:       map[string]string{nodeKeyFile: `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"key3"}}`},
		},
	}}

	var crd cosmosv1.CosmosFullNode
	crd.Name = "dydx"
	crd.Namespace = namespace
	crd.Spec.Replicas = 2 // Reduced from 3 to 2

	collector := NewNodeKeyCollector(&mClient)

	nodeKeys, err := collector.Collect(ctx, &crd)

	require.NoError(t, err)
	require.Len(t, nodeKeys, 2)
	require.Contains(t, nodeKeys, client.ObjectKey{Name: "dydx-0", Namespace: namespace})
	require.Contains(t, nodeKeys, client.ObjectKey{Name: "dydx-1", Namespace: namespace})
	require.NotContains(t, nodeKeys, client.ObjectKey{Name: "dydx-2", Namespace: namespace})
}
