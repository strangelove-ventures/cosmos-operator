package fullnode

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
)

const nodeKeyFile = "node_key.json"

// BuildNodeKeySecrets builds the node key secrets for the given CRD.
// If the secret already has a node key, it is reused.
// Returns an error if a new node key cannot be serialized. (Should never happen.)
func BuildNodeKeySecrets(existing []*corev1.Secret, crd *cosmosv1.CosmosFullNode) ([]diff.Resource[*corev1.Secret], error) {
	secrets := make([]diff.Resource[*corev1.Secret], crd.Spec.Replicas)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		var s corev1.Secret
		s.Name = nodeKeySecretName(crd, i)
		s.Namespace = crd.Namespace
		s = *kube.FindOrDefaultCopy(existing, &s)

		var secret corev1.Secret
		secret.Name = s.Name
		secret.Namespace = s.Namespace
		secret.Kind = "Secret"
		secret.APIVersion = "v1"
		secret.Data = s.Data

		secret.Labels = defaultLabels(crd,
			kube.InstanceLabel, instanceName(crd, i),
		)

		secret.Immutable = ptr(true)
		secret.Type = corev1.SecretTypeOpaque

		// Create node key if it doesn't exist
		if secret.Data[nodeKeyFile] == nil {
			nk, err := randNodeKey()
			if err != nil {
				return nil, err
			}
			secret.Data = map[string][]byte{
				nodeKeyFile: nk,
			}
		}

		secrets[i] = diff.Adapt(&secret, i)
	}
	return secrets, nil
}

type NodeKey struct {
	PrivKey NodeKeyPrivKey `json:"priv_key"`
}

type NodeKeyPrivKey struct {
	Type  string             `json:"type"`
	Value ed25519.PrivateKey `json:"value"`
}

func (nk NodeKey) ID() string {
	pub := nk.PrivKey.Value.Public()
	hash := sha256.Sum256(pub.(ed25519.PublicKey))
	return hex.EncodeToString(hash[:20])
}

func randNodeKey() ([]byte, error) {
	_, pk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 node key: %w", err)
	}
	return json.Marshal(NodeKey{
		PrivKey: NodeKeyPrivKey{
			Type:  "tendermint/PrivKeyEd25519",
			Value: pk,
		},
	})
}

func nodeKeySecretName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return kube.ToName(fmt.Sprintf("%s-node-key-%d", appName(crd), ordinal))
}
