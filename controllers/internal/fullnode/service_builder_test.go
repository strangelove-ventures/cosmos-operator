package fullnode

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestBuildServices(t *testing.T) {
	t.Parallel()

	t.Run("p2p services", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = "terra"
		crd.Namespace = "test"
		crd.Spec.ChainConfig.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		svcs := BuildServices(&crd)

		require.Equal(t, 4, len(svcs)) // Includes single rpc service.

		p2p := svcs[0]
		require.Equal(t, "terra-testnet-fullnode-p2p-0", p2p.Name)
		require.Equal(t, "test", p2p.Namespace)

		require.NotEmpty(t, p2p.Labels[kube.RevisionLabel])
		delete(p2p.Labels, kube.RevisionLabel)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmosfullnode",
			"app.kubernetes.io/name":       "terra-testnet-fullnode",
			"app.kubernetes.io/component":  "p2p",
			"app.kubernetes.io/version":    "v6.0.0",
			"app.kubernetes.io/instance":   "terra-testnet-fullnode-0",
			"cosmos.strange.love/network":  "testnet",
		}
		require.Equal(t, wantLabels, p2p.Labels)

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

		require.Equal(t, wantSpec, p2p.Spec)

		p2p = svcs[1]
		require.Equal(t, "terra-testnet-fullnode-p2p-1", p2p.Name)
	})

	t.Run("rpc service", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 1
		crd.Name = "terra"
		crd.Namespace = "test"
		crd.Spec.ChainConfig.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		svcs := BuildServices(&crd)

		require.Equal(t, 2, len(svcs)) // Includes single p2p service.

		rpc := svcs[1]
		require.Equal(t, "terra-testnet-fullnode-rpc", rpc.Name)
		require.Equal(t, "test", rpc.Namespace)
		require.Equal(t, corev1.ServiceTypeClusterIP, rpc.Spec.Type)
		require.Equal(t, map[string]string{"app.kubernetes.io/name": "terra-testnet-fullnode"}, rpc.Spec.Selector)

		require.NotEmpty(t, rpc.Labels[kube.RevisionLabel])
		delete(rpc.Labels, kube.RevisionLabel)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmosfullnode",
			"app.kubernetes.io/name":       "terra-testnet-fullnode",
			"app.kubernetes.io/component":  "rpc",
			"app.kubernetes.io/version":    "v6.0.0",
			"cosmos.strange.love/network":  "testnet",
		}
		require.Equal(t, wantLabels, rpc.Labels)

		require.Equal(t, 5, len(rpc.Spec.Ports))
		// All ports minus prometheus and p2p.
		want := []corev1.ServicePort{
			{
				Name:       "api",
				Protocol:   corev1.ProtocolTCP,
				Port:       1317,
				TargetPort: intstr.FromString("api"),
			},
			{
				Name:       "rosetta",
				Protocol:   corev1.ProtocolTCP,
				Port:       8080,
				TargetPort: intstr.FromString("rosetta"),
			},
			{
				Name:       "grpc",
				Protocol:   corev1.ProtocolTCP,
				Port:       9090,
				TargetPort: intstr.FromString("grpc"),
			},
			{
				Name:       "rpc",
				Protocol:   corev1.ProtocolTCP,
				Port:       26657,
				TargetPort: intstr.FromString("rpc"),
			},
			{
				Name:       "grpc-web",
				Protocol:   corev1.ProtocolTCP,
				Port:       9091,
				TargetPort: intstr.FromString("grpc-web"),
			},
		}

		require.Equal(t, want, rpc.Spec.Ports)
	})

	t.Run("rpc service with overrides", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 0
		crd.Name = "terra"
		crd.Namespace = "test"
		crd.Spec.ChainConfig.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		svcs := BuildServices(&crd)

		require.Equal(t, 2, len(svcs)) // Includes single p2p service.

		rpc := svcs[1]
		require.Equal(t, "terra-testnet-fullnode-rpc", rpc.Name)
		require.Equal(t, "test", rpc.Namespace)
		require.Equal(t, corev1.ServiceTypeClusterIP, rpc.Spec.Type)
		require.Equal(t, map[string]string{"app.kubernetes.io/name": "terra-testnet-fullnode"}, rpc.Spec.Selector)

		require.Equal(t, 5, len(rpc.Spec.Ports))
		// All ports minus prometheus and p2p.
		want := []corev1.ServicePort{
			{
				Name:       "api",
				Protocol:   corev1.ProtocolTCP,
				Port:       1317,
				TargetPort: intstr.FromString("api"),
			},
			{
				Name:       "rosetta",
				Protocol:   corev1.ProtocolTCP,
				Port:       8080,
				TargetPort: intstr.FromString("rosetta"),
			},
			{
				Name:       "grpc",
				Protocol:   corev1.ProtocolTCP,
				Port:       9090,
				TargetPort: intstr.FromString("grpc"),
			},
			{
				Name:       "rpc",
				Protocol:   corev1.ProtocolTCP,
				Port:       26657,
				TargetPort: intstr.FromString("rpc"),
			},
			{
				Name:       "grpc-web",
				Protocol:   corev1.ProtocolTCP,
				Port:       9091,
				TargetPort: intstr.FromString("grpc-web"),
			},
		}

		require.Equal(t, want, rpc.Spec.Ports)
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

func FuzzBuildServices(f *testing.F) {
	crd := defaultCRD()
	crd.Spec.Replicas = 1

	f.Add("abcd@1.2.3.4:26656", "NodePort")
	f.Fuzz(func(t *testing.T, image, svcType string) {
		crd.Spec.PodTemplate.Image = image
		uniq := func(svcs []*corev1.Service) []string {
			return lo.Uniq(lo.Map(svcs, func(s *corev1.Service, _ int) string {
				return s.Labels[kube.RevisionLabel]
			}))
		}
		svcs1 := BuildServices(&crd)
		require.Len(t, uniq(svcs1), 1)
		require.NotZero(t, uniq(svcs1)[0])

		svcs2 := BuildServices(&crd)
		require.Equal(t, uniq(svcs1), uniq(svcs2))

		crd.Spec.RPCServiceTemplate.Type = ptr(corev1.ServiceType(svcType))
		svcs3 := BuildServices(&crd)
		require.NotEqual(t, uniq(svcs1), uniq(svcs3))
	})
}
