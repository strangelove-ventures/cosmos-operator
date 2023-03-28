package fullnode

import (
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
)

func BuildNodeKeySecrets(existing []*corev1.Secret, crd *cosmosv1.CosmosFullNode) []*corev1.Secret {
	secrets := make([]*corev1.Secret, crd.Spec.Replicas)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		var s corev1.Secret
		s.Name = nodeKeySecretName(crd, i)
		s.Namespace = crd.Namespace
		s.Kind = "Secret"
		s.APIVersion = "v1"
		s.Labels = defaultLabels(crd)
		s.Labels[kube.InstanceLabel] = instanceName(crd, i)
		s.Annotations = make(map[string]string)
		s.Annotations[kube.OrdinalAnnotation] = kube.ToIntegerValue(i)

		s.Immutable = ptr(true)
		s.Type = corev1.SecretTypeOpaque

		secrets[i] = &s
	}
	return secrets
}

func nodeKeySecretName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return kube.ToName(fmt.Sprintf("%s-node-key-%d", appName(crd), ordinal))
}
