package fullnode

import (
	"strings"
	"testing"

	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestBuildServices(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = "terra"
		crd.Namespace = "test"
		crd.Spec.ChainConfig.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		svcs := BuildServices(&crd)

		require.Equal(t, 3, len(svcs))

		got := svcs[0]
		require.Equal(t, "terra-testnet-fullnode-p2p-0", got.Name)
		require.Equal(t, "test", got.Namespace)

		require.NotEmpty(t, got.Labels[kube.RevisionLabel])
		delete(got.Labels, kube.RevisionLabel)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmosfullnode",
			"app.kubernetes.io/name":       "terra-testnet-fullnode",
			"app.kubernetes.io/component":  "p2p",
			"app.kubernetes.io/version":    "v6.0.0",
			"app.kubernetes.io/instance":   "terra-testnet-fullnode-0",
			"cosmos.strange.love/network":  "testnet",
		}
		require.Equal(t, wantLabels, got.Labels)

		wantSpec := corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "p2p",
					Protocol:   corev1.ProtocolTCP,
					Port:       26656,
					TargetPort: intstr.FromString("p2p"),
				},
			},
			Selector:              map[string]string{"app.kubernetes.io/instance": "terra-testnet-fullnode-0"},
			Type:                  "LoadBalancer",
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
		}

		require.Equal(t, wantSpec, got.Spec)

		got = svcs[1]
		require.Equal(t, "terra-testnet-fullnode-p2p-1", got.Name)
	})

	t.Run("long name", func(t *testing.T) {
		crd := defaultCRD()
		name := strings.Repeat("Long", 500)
		crd.Name = name

		for _, svc := range BuildServices(&crd) {
			RequireValidMetadata(t, svc)
		}
	})
}
