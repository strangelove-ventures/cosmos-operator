package fullnode

import (
	"fmt"
	"strings"
	"testing"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
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

		svcs := BuildServices(&crd)

		require.Equal(t, 4, len(svcs)) // 3 p2p services + 1 rpc service

		for i, svc := range svcs[:3] {
			p2p := svc.Object()
			require.Equal(t, fmt.Sprintf("terra-p2p-%d", i), p2p.Name)
			require.Equal(t, "test", p2p.Namespace)

			wantLabels := map[string]string{
				"app.kubernetes.io/created-by": "cosmos-operator",
				"app.kubernetes.io/name":       "terra",
				"app.kubernetes.io/component":  "p2p",
				"app.kubernetes.io/version":    "v6.0.0",
				"app.kubernetes.io/instance":   fmt.Sprintf("terra-%d", i),
				"cosmos.strange.love/network":  "testnet",
				"cosmos.strange.love/type":     "FullNode",
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
				Selector: map[string]string{"app.kubernetes.io/instance": fmt.Sprintf("terra-%d", i)},
				Type:     corev1.ServiceTypeClusterIP,
			}
			// By default, expose the first p2p service publicly.
			if i == 0 {
				wantSpec.Type = corev1.ServiceTypeLoadBalancer
				wantSpec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
			}

			require.Equal(t, wantSpec, p2p.Spec)
		}
	})

	t.Run("p2p services", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = "terra"
		crd.Namespace = "test"
		crd.Spec.ChainSpec.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		crd.Spec.Ordinals.Start = 2

		svcs := BuildServices(&crd)

		require.Equal(t, 4, len(svcs)) // 3 p2p services + 1 rpc service

		for i := 0; i < int(crd.Spec.Replicas); i++ {
			ordinal := crd.Spec.Ordinals.Start + int32(i)
			p2p := svcs[i].Object()
			require.Equal(t, fmt.Sprintf("terra-p2p-%d", ordinal), p2p.Name)
			require.Equal(t, "test", p2p.Namespace)

			wantLabels := map[string]string{
				"app.kubernetes.io/created-by": "cosmos-operator",
				"app.kubernetes.io/name":       "terra",
				"app.kubernetes.io/component":  "p2p",
				"app.kubernetes.io/version":    "v6.0.0",
				"app.kubernetes.io/instance":   fmt.Sprintf("terra-%d", ordinal),
				"cosmos.strange.love/network":  "testnet",
				"cosmos.strange.love/type":     "FullNode",
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
				Selector: map[string]string{"app.kubernetes.io/instance": fmt.Sprintf("terra-%d", ordinal)},
				Type:     corev1.ServiceTypeClusterIP,
			}
			// By default, expose the first p2p service publicly.
			if i == 0 {
				wantSpec.Type = corev1.ServiceTypeLoadBalancer
				wantSpec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
			}

			require.Equal(t, wantSpec, p2p.Spec)
		}
	})

	t.Run("p2p max external addresses", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Spec.Service.MaxP2PExternalAddresses = ptr(int32(2))

		svcs := BuildServices(&crd)

		gotP2P := lo.Filter(svcs, func(s diff.Resource[*corev1.Service], _ int) bool {
			return s.Object().Labels[kube.ComponentLabel] == "p2p"
		})

		require.Equal(t, 3, len(gotP2P))
		for i, svc := range gotP2P[:2] {
			p2p := svc.Object()
			require.Equal(t, corev1.ServiceTypeLoadBalancer, p2p.Spec.Type, i)
			require.Equal(t, corev1.ServiceExternalTrafficPolicyTypeLocal, p2p.Spec.ExternalTrafficPolicy, i)
		}

		got := gotP2P[2].Object()
		require.Equal(t, corev1.ServiceTypeClusterIP, got.Spec.Type)
		require.Empty(t, got.Spec.ExternalTrafficPolicy)
	})

	t.Run("zero p2p max external addresses", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Spec.Service.MaxP2PExternalAddresses = ptr(int32(0))
		// These overrides should be ignored.
		crd.Spec.Service.P2PTemplate = cosmosv1.ServiceOverridesSpec{
			Metadata: cosmosv1.Metadata{
				Labels: map[string]string{"test": "should not see me"},
			},
			Type:                  ptr(corev1.ServiceTypeNodePort),
			ExternalTrafficPolicy: ptr(corev1.ServiceExternalTrafficPolicyTypeLocal),
		}

		svcs := BuildServices(&crd)

		gotP2P := lo.Filter(svcs, func(s diff.Resource[*corev1.Service], _ int) bool {
			return s.Object().Labels[kube.ComponentLabel] == "p2p"
		})

		require.Equal(t, 3, len(gotP2P))
		for i, svc := range gotP2P {
			p2p := svc.Object()
			require.Empty(t, p2p.Labels["test"])
			require.Equal(t, corev1.ServiceTypeClusterIP, p2p.Spec.Type, i)
			require.Empty(t, p2p.Spec.ExternalTrafficPolicy, i)
		}
	})

	t.Run("p2p services with overrides", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 2
		crd.Name = "terra"
		crd.Spec.Service.MaxP2PExternalAddresses = ptr(int32(2))
		crd.Spec.Service.P2PTemplate = cosmosv1.ServiceOverridesSpec{
			Metadata: cosmosv1.Metadata{
				Labels:      map[string]string{"test": "value1", "app.kubernetes.io/name": "should not see me"},
				Annotations: map[string]string{"test": "value2", "app.kubernetes.io/ordinal": "should not see me"},
			},
			Type:                  ptr(corev1.ServiceTypeNodePort),
			ExternalTrafficPolicy: ptr(corev1.ServiceExternalTrafficPolicyTypeLocal),
		}
		svcs := BuildServices(&crd)

		require.Equal(t, 3, len(svcs)) // 2 p2p services + 1 rpc service

		for i, svc := range svcs[:2] {
			p2p := svc.Object()
			require.Equal(t, fmt.Sprintf("terra-p2p-%d", i), p2p.Name)

			require.Equal(t, "value1", p2p.Labels["test"])
			require.NotEqual(t, "should not see me", p2p.Labels["app.kubernetes.io/name"])

			require.Equal(t, "value2", p2p.Annotations["test"])
			require.NotEqual(t, "should not see me", p2p.Labels["app.kubernetes.io/ordinal"])

			wantSpec := corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "p2p",
						Protocol:   corev1.ProtocolTCP,
						Port:       26656,
						TargetPort: intstr.FromString("p2p"),
					},
				},
				Selector:              map[string]string{"app.kubernetes.io/instance": fmt.Sprintf("terra-%d", i)},
				Type:                  corev1.ServiceTypeNodePort,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			}

			require.Equal(t, wantSpec, p2p.Spec)
		}
	})

	t.Run("rpc service", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 1
		crd.Name = "terra"
		crd.Namespace = "test"
		crd.Spec.ChainSpec.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		svcs := BuildServices(&crd)

		require.Equal(t, 2, len(svcs)) // Includes single p2p service.

		rpc := svcs[1].Object()
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
			"cosmos.strange.love/type":     "FullNode",
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
		crd.Spec.ChainSpec.Network = "testnet"
		crd.Spec.PodTemplate.Image = "terra:v6.0.0"
		crd.Spec.Service.RPCTemplate = cosmosv1.ServiceOverridesSpec{
			Metadata: cosmosv1.Metadata{
				Labels:      map[string]string{"label": "value", "app.kubernetes.io/name": "should not see me"},
				Annotations: map[string]string{"test": "value"},
			},
			Type:                  ptr(corev1.ServiceTypeNodePort),
			ExternalTrafficPolicy: ptr(corev1.ServiceExternalTrafficPolicyTypeLocal),
		}
		svcs := BuildServices(&crd)

		rpc := svcs[0].Object()
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

		svcs := BuildServices(&crd)

		for _, svc := range svcs {
			test.RequireValidMetadata(t, svc.Object())
		}
	})

	test.HasTypeLabel(t, func(crd cosmosv1.CosmosFullNode) []map[string]string {
		svcs := BuildServices(&crd)
		labels := make([]map[string]string, 0)
		for _, svc := range svcs {
			labels = append(labels, svc.Object().Labels)
		}
		return labels
	})
}
