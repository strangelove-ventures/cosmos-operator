package fullnode

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
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

func TestNodeKeyCollector_SpecNodeKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const namespace = "strangelove"

	t.Run("valid spec node keys", func(t *testing.T) {
		// Generate valid ed25519 private keys for testing
		_, pk1, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		_, pk2, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)

		nodeKey1Base64 := base64.StdEncoding.EncodeToString(pk1)
		nodeKey2Base64 := base64.StdEncoding.EncodeToString(pk2)

		var mClient nodeKeyMockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{}}

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 2
		crd.Spec.NodeKeys = []string{nodeKey1Base64, nodeKey2Base64}

		collector := NewNodeKeyCollector(&mClient)

		nodeKeys, err := collector.Collect(ctx, &crd)

		require.NoError(t, err)
		require.Len(t, nodeKeys, 2)

		// Verify the node keys match what we provided
		nk1 := nodeKeys[client.ObjectKey{Name: "dydx-0", Namespace: namespace}]
		nk2 := nodeKeys[client.ObjectKey{Name: "dydx-1", Namespace: namespace}]

		require.Equal(t, pk1, nk1.NodeKey.PrivKey.Value)
		require.Equal(t, pk2, nk2.NodeKey.PrivKey.Value)
	})

	t.Run("partial spec node keys - generates missing keys", func(t *testing.T) {
		// Only provide 1 key for 3 replicas
		_, pk1, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		nodeKey1Base64 := base64.StdEncoding.EncodeToString(pk1)

		var mClient nodeKeyMockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{}}

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 3
		crd.Spec.NodeKeys = []string{nodeKey1Base64}

		collector := NewNodeKeyCollector(&mClient)

		nodeKeys, err := collector.Collect(ctx, &crd)

		require.NoError(t, err)
		require.Len(t, nodeKeys, 3)

		// First key should match what we provided
		nk1 := nodeKeys[client.ObjectKey{Name: "dydx-0", Namespace: namespace}]
		require.Equal(t, pk1, nk1.NodeKey.PrivKey.Value)

		// Other keys should be generated (different from the first)
		nk2 := nodeKeys[client.ObjectKey{Name: "dydx-1", Namespace: namespace}]
		nk3 := nodeKeys[client.ObjectKey{Name: "dydx-2", Namespace: namespace}]
		require.NotEqual(t, pk1, nk2.NodeKey.PrivKey.Value)
		require.NotEqual(t, pk1, nk3.NodeKey.PrivKey.Value)
		require.NotEqual(t, nk2.NodeKey.PrivKey.Value, nk3.NodeKey.PrivKey.Value)
	})

	t.Run("configmap node key should take precedence than spec node keys", func(t *testing.T) {
		// Generate a new key for spec
		_, specPk, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		specKeyBase64 := base64.StdEncoding.EncodeToString(specPk)

		// ConfigMap has existing key
		existingNodeKey := `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"HBX8VFQ4OdWfOwIOR7jj0af8mVHik5iGW9o1xnn4vRltk1HmwQS2LLGrMPVS2LIUO9BUqmZ1Pjt+qM8x0ibHxQ=="}}`

		var mClient nodeKeyMockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "dydx-0", Namespace: namespace},
				Data:       map[string]string{nodeKeyFile: existingNodeKey},
			},
		}}

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1
		crd.Spec.NodeKeys = []string{specKeyBase64}

		collector := NewNodeKeyCollector(&mClient)

		nodeKeys, err := collector.Collect(ctx, &crd)

		require.NoError(t, err)
		require.Len(t, nodeKeys, 1)

		// Should use configmap key, not spec key
		nk := nodeKeys[client.ObjectKey{Name: "dydx-0", Namespace: namespace}]
		require.Equal(t, existingNodeKey, string(nk.MarshaledNodeKey))

		// Verify it's not using the spec key
		require.NotEqual(t, specPk, nk.NodeKey.PrivKey.Value)
	})

	t.Run("invalid base64 node key", func(t *testing.T) {
		var mClient nodeKeyMockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{}}

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1
		crd.Spec.NodeKeys = []string{"invalid-base64!@#"}

		collector := NewNodeKeyCollector(&mClient)

		_, err := collector.Collect(ctx, &crd)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid node key")
	})

	t.Run("wrong key size in spec", func(t *testing.T) {
		// Create a key with wrong size (32 bytes instead of 64)
		shortKey := make([]byte, 32)
		shortKeyBase64 := base64.StdEncoding.EncodeToString(shortKey)

		var mClient nodeKeyMockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{}}

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 1
		crd.Spec.NodeKeys = []string{shortKeyBase64}

		collector := NewNodeKeyCollector(&mClient)

		_, err := collector.Collect(ctx, &crd)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid node key")
		require.Contains(t, err.Error(), "invalid key size")
	})

	t.Run("spec node keys with non-zero ordinals", func(t *testing.T) {
		_, pk1, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		_, pk2, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)

		nodeKey1Base64 := base64.StdEncoding.EncodeToString(pk1)
		nodeKey2Base64 := base64.StdEncoding.EncodeToString(pk2)

		var mClient nodeKeyMockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{}}

		var crd cosmosv1.CosmosFullNode
		crd.Name = "dydx"
		crd.Namespace = namespace
		crd.Spec.Replicas = 2
		crd.Spec.Ordinals.Start = 5
		crd.Spec.NodeKeys = []string{nodeKey1Base64, nodeKey2Base64}

		collector := NewNodeKeyCollector(&mClient)

		nodeKeys, err := collector.Collect(ctx, &crd)

		require.NoError(t, err)
		require.Len(t, nodeKeys, 2)

		// Verify the node keys are mapped correctly to ordinals 5 and 6
		nk1 := nodeKeys[client.ObjectKey{Name: "dydx-5", Namespace: namespace}]
		nk2 := nodeKeys[client.ObjectKey{Name: "dydx-6", Namespace: namespace}]

		require.Equal(t, pk1, nk1.NodeKey.PrivKey.Value)
		require.Equal(t, pk2, nk2.NodeKey.PrivKey.Value)
	})
}

func TestBase64StrToNodeKey(t *testing.T) {
	t.Parallel()

	t.Run("valid base64 ed25519 key", func(t *testing.T) {
		_, pk, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)

		keyBase64 := base64.StdEncoding.EncodeToString(pk)

		nodeKey, err := base64StrToNodeKey(keyBase64)
		require.NoError(t, err)
		require.NotNil(t, nodeKey)
		require.Equal(t, "tendermint/PrivKeyEd25519", nodeKey.PrivKey.Type)
		require.Equal(t, pk, nodeKey.PrivKey.Value)
	})

	t.Run("invalid base64", func(t *testing.T) {
		_, err := base64StrToNodeKey("invalid-base64!@#")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode base64 string")
	})

	t.Run("wrong key size", func(t *testing.T) {
		// Create key with wrong size
		shortKey := make([]byte, 32)
		keyBase64 := base64.StdEncoding.EncodeToString(shortKey)

		_, err := base64StrToNodeKey(keyBase64)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid key size")
	})
}
