package fullnode

import (
	"fmt"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const maxP2PServiceDefault = int32(1)

// BuildServices returns a list of services given the crd.
//
// Creates services based on the node type:
// - For regular nodes: Creates 1 RPC service and 1 p2p service per pod (replicas + 1 total)
// - For sentry nodes: Creates 1 RPC service and 2 p2p services per pod (replicas * 2 + 1 total)
//
// P2P services diverge from traditional web and kubernetes architecture which typically uses a single
// service backed by multiple pods. This is necessary because:
//  1. Pods may be in various states even with proper readiness probes
//  2. We need to prevent confusion or disruption in peer exchange (PEX) within CometBFT
//  3. Using a single p2p service could lead to misinterpreted byzantine behavior if an outside peer
//     discovers a pod out of sync after previously connecting to a synced pod through the same address
//
// Services are created with either LoadBalancer (for external access) or ClusterIP type, controlled
// by MaxP2PExternalAddresses setting.
func BuildServices(crd *cosmosv1.CosmosFullNode) []diff.Resource[*corev1.Service] {
	max := maxP2PServiceDefault
	if v := crd.Spec.Service.MaxP2PExternalAddresses; v != nil {
		max = *v
	}
	maxExternal := lo.Clamp(max, 0, crd.Spec.Replicas)
	totalServices := crd.Spec.Replicas + 1
	if crd.Spec.Type == cosmosv1.Sentry {
		totalServices = (crd.Spec.Replicas * 2) + 1
	}

	svcs := make([]diff.Resource[*corev1.Service], 0, totalServices)

	startOrdinal := crd.Spec.Ordinals.Start
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		ordinal := startOrdinal + i
		var svc corev1.Service
		svc.Name = p2pServiceName(crd, ordinal)
		svc.Namespace = crd.Namespace
		svc.Kind = "Service"
		svc.APIVersion = "v1"
		svc.Labels = defaultLabels(crd,
			kube.InstanceLabel, instanceName(crd, ordinal),
			kube.ComponentLabel, "p2p",
		)
		svc.Annotations = map[string]string{}
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "p2p",
				Protocol:   corev1.ProtocolTCP,
				Port:       p2pPort,
				TargetPort: intstr.FromString("p2p"),
			},
		}
		svc.Spec.Selector = map[string]string{kube.InstanceLabel: instanceName(crd, ordinal)}
		if i < maxExternal {
			preserveMergeInto(svc.Labels, crd.Spec.Service.P2PTemplate.Metadata.Labels)
			preserveMergeInto(svc.Annotations, crd.Spec.Service.P2PTemplate.Metadata.Annotations)
			svc.Spec.Type = *valOrDefault(crd.Spec.Service.P2PTemplate.Type, ptr(corev1.ServiceTypeLoadBalancer))
			svc.Spec.ExternalTrafficPolicy = *valOrDefault(crd.Spec.Service.P2PTemplate.ExternalTrafficPolicy, ptr(corev1.ServiceExternalTrafficPolicyTypeLocal))
		} else {
			svc.Spec.Type = corev1.ServiceTypeClusterIP
			svc.Spec.ClusterIP = *valOrDefault(crd.Spec.Service.P2PTemplate.ClusterIP, ptr(""))
		}
		svcs = append(svcs, diff.Adapt(&svc, len(svcs)))
	}

	// Add sentry services if needed
	if crd.Spec.Type == cosmosv1.Sentry {
		for i := int32(0); i < crd.Spec.Replicas; i++ {
			ordinal := startOrdinal + i
			var svc corev1.Service
			svc.Name = sentryServiceName(crd, ordinal)
			svc.Namespace = crd.Namespace
			svc.Kind = "Service"
			svc.APIVersion = "v1"

			svc.Labels = defaultLabels(crd,
				kube.InstanceLabel, instanceName(crd, ordinal),
				kube.ComponentLabel, "cosmos-sentry",
			)
			svc.Annotations = map[string]string{}

			svc.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "sentry-privval",
					Protocol:   corev1.ProtocolTCP,
					Port:       privvalPort,
					TargetPort: intstr.FromString("privval"),
				},
			}
			svc.Spec.Selector = map[string]string{kube.InstanceLabel: instanceName(crd, ordinal)}

			preserveMergeInto(svc.Labels, crd.Spec.Service.P2PTemplate.Metadata.Labels)
			preserveMergeInto(svc.Annotations, crd.Spec.Service.P2PTemplate.Metadata.Annotations)
			svc.Spec.Type = corev1.ServiceTypeClusterIP
			svc.Spec.PublishNotReadyAddresses = true

			svcs = append(svcs, diff.Adapt(&svc, len(svcs)))
		}
	}

	// Add RPC service
	svcs = append(svcs, diff.Adapt(rpcService(crd), len(svcs)))

	return svcs
}

func rpcService(crd *cosmosv1.CosmosFullNode) *corev1.Service {
	var svc corev1.Service
	svc.Name = rpcServiceName(crd)
	svc.Namespace = crd.Namespace
	svc.Kind = "Service"
	svc.APIVersion = "v1"
	svc.Labels = defaultLabels(crd,
		kube.ComponentLabel, "rpc",
	)
	svc.Annotations = map[string]string{}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "api",
			Protocol:   corev1.ProtocolTCP,
			Port:       apiPort,
			TargetPort: intstr.FromString("api"),
		},
		{
			Name:       "rosetta",
			Protocol:   corev1.ProtocolTCP,
			Port:       rosettaPort,
			TargetPort: intstr.FromString("rosetta"),
		},
		{
			Name:       "grpc",
			Protocol:   corev1.ProtocolTCP,
			Port:       grpcPort,
			TargetPort: intstr.FromString("grpc"),
		},
		{
			Name:       "rpc",
			Protocol:   corev1.ProtocolTCP,
			Port:       rpcPort,
			TargetPort: intstr.FromString("rpc"),
		},
		{
			Name:       "grpc-web",
			Protocol:   corev1.ProtocolTCP,
			Port:       grpcWebPort,
			TargetPort: intstr.FromString("grpc-web"),
		},
	}
	svc.Spec.Selector = map[string]string{kube.NameLabel: appName(crd)}
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	rpcSpec := crd.Spec.Service.RPCTemplate
	preserveMergeInto(svc.Labels, rpcSpec.Metadata.Labels)
	preserveMergeInto(svc.Annotations, rpcSpec.Metadata.Annotations)
	svc.Spec.Ports = append(svc.Spec.Ports, rpcSpec.Ports...)
	kube.NormalizeMetadata(&svc.ObjectMeta)
	if v := rpcSpec.ExternalTrafficPolicy; v != nil {
		svc.Spec.ExternalTrafficPolicy = *v
	}
	if v := rpcSpec.Type; v != nil {
		svc.Spec.Type = *v
	}
	if v := rpcSpec.ClusterIP; v != nil {
		svc.Spec.ClusterIP = *v
	}
	return &svc
}

func p2pServiceName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return fmt.Sprintf("%s-p2p-%d", appName(crd), ordinal)
}

func sentryServiceName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return fmt.Sprintf("%s-privval-%d", appName(crd), ordinal)
}

func rpcServiceName(crd *cosmosv1.CosmosFullNode) string {
	return fmt.Sprintf("%s-rpc", appName(crd))
}
