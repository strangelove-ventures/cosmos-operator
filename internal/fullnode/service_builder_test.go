package fullnode

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/strangelove-ventures/cosmos-operator/internal/test"
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
		crd.Spec.ChainSpec.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		svcs, err := BuildServices(nil, &crd)
		require.NoError(t, err)

		require.Equal(t, 2, len(svcs)) // Includes single rpc service.

		p2p := svcs[0]
		require.Equal(t, "terra-p2p-0", p2p.Name)
		require.Equal(t, "test", p2p.Namespace)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/name":       "terra",
			"app.kubernetes.io/component":  "p2p",
			"app.kubernetes.io/version":    "v6.0.0",
			"app.kubernetes.io/instance":   "terra-0",
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
			Selector:              map[string]string{"app.kubernetes.io/instance": "terra-0"},
			Type:                  "LoadBalancer",
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
		}

		require.Equal(t, wantSpec, p2p.Spec)
	})

	t.Run("p2p max external addresses", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 10

		for i := 0; i < 5; i++ {
			crd.Spec.Service.MaxP2PExternalAddresses = ptr(int32(i))
			svcs, err := BuildServices(nil, &crd)
			require.NoError(t, err)

			got := lo.Filter(svcs, func(s *corev1.Service, _ int) bool {
				return s.Labels[kube.ComponentLabel] == "p2p"
			})

			require.Equal(t, i, len(got))
		}

		crd.Spec.Replicas = 1
		crd.Spec.Service.MaxP2PExternalAddresses = ptr(int32(2))

		svcs, err := BuildServices(nil, &crd)
		require.NoError(t, err)

		got := lo.Filter(svcs, func(s *corev1.Service, _ int) bool {
			return s.Labels[kube.ComponentLabel] == "p2p"
		})

		require.Equal(t, 1, len(got))
	})

	t.Run("rpc service", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 1
		crd.Name = "terra"
		crd.Namespace = "test"
		crd.Spec.ChainSpec.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		svcs, err := BuildServices(nil, &crd)

		require.NoError(t, err)

		require.Equal(t, 2, len(svcs)) // Includes single p2p service.

		rpc := svcs[1]
		require.Equal(t, "terra-rpc", rpc.Name)
		require.Equal(t, "test", rpc.Namespace)
		require.Equal(t, corev1.ServiceTypeClusterIP, rpc.Spec.Type)
		require.Equal(t, map[string]string{"app.kubernetes.io/name": "terra"}, rpc.Spec.Selector)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/name":       "terra",
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

	t.Run("preserves existing", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 2
		crd.Name = "terra"

		svcs, err := BuildServices(nil, &crd)
		require.NoError(t, err)

		for i := range svcs {
			ports := svcs[i].Spec.Ports
			for p := range ports {
				ports[p].NodePort = 12345 // Set by kubernetes
			}
		}

		svcs2, err := BuildServices(svcs, &crd)
		require.NoError(t, err)
		require.NotSame(t, svcs, svcs2)
		require.Equal(t, valSlice(svcs), valSlice(svcs2))
	})

	t.Run("rpc service with overrides", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 0
		crd.Name = "terra"
		crd.Namespace = "test"
		crd.Spec.ChainSpec.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		crd.Spec.Service.RPCTemplate = cosmosv1.RPCServiceSpec{
			Metadata: cosmosv1.Metadata{
				Labels:      map[string]string{"label": "value", "app.kubernetes.io/name": "should not see me"},
				Annotations: map[string]string{"test": "value"},
			},
			Type:                  ptr(corev1.ServiceTypeNodePort),
			ExternalTrafficPolicy: ptr(corev1.ServiceExternalTrafficPolicyTypeLocal),
		}
		svcs, err := BuildServices(nil, &crd)
		require.NoError(t, err)

		rpc := svcs[0]
		require.Equal(t, map[string]string{"test": "value"}, rpc.Annotations)

		require.Equal(t, "value", rpc.Labels["label"])
		require.Equal(t, "terra", rpc.Labels["app.kubernetes.io/name"])

		require.Equal(t, corev1.ServiceExternalTrafficPolicyTypeLocal, rpc.Spec.ExternalTrafficPolicy)
		require.Equal(t, corev1.ServiceTypeNodePort, rpc.Spec.Type)
	})

	t.Run("long name", func(t *testing.T) {
		crd := defaultCRD()
		name := strings.Repeat("Long", 500)
		crd.Name = name

		svcs, err := BuildServices(nil, &crd)
		require.NoError(t, err)

		for _, svc := range svcs {
			test.RequireValidMetadata(t, svc)
		}
	})
}
