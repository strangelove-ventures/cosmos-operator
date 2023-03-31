package fullnode

import (
	"fmt"

	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
)

const nodeKeySecret = "node_key.json"

// BuildNodeKeySecrets builds the node key secrets for the given CRD.
// If the secret already has a node key, it is reused.
// Returns an error if a new node key cannot be serialized. (Should never happen.)
func BuildNodeKeySecrets(existing []*corev1.Secret, crd *cosmosv1.CosmosFullNode) ([]*corev1.Secret, error) {
	secrets := make([]*corev1.Secret, crd.Spec.Replicas)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		var s corev1.Secret
		s.Name = nodeKeySecretName(crd, i)
		s.Namespace = crd.Namespace
		s = *kube.FindOrDefault(existing, &s)

		s.Kind = "Secret"
		s.APIVersion = "v1"
		s.Labels = defaultLabels(crd)
		s.Labels[kube.InstanceLabel] = instanceName(crd, i)
		s.Annotations = make(map[string]string)
		s.Annotations[kube.OrdinalAnnotation] = kube.ToIntegerValue(i)

		s.Immutable = ptr(true)
		s.Type = corev1.SecretTypeOpaque

		// Create node key if it doesn't exist
		if s.Data[nodeKeySecret] == nil {
			nodeKey := p2p.NodeKey{PrivKey: ed25519.GenPrivKey()}
			b, err := cmtjson.Marshal(nodeKey)
			if err != nil {
				return nil, err
			}
			s.Data = map[string][]byte{
				nodeKeySecret: b,
			}
		}

		secrets[i] = &s
	}
	return secrets, nil
}

func nodeKeySecretName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return kube.ToName(fmt.Sprintf("%s-node-key-%d", appName(crd), ordinal))
}
