package fullnode

import (
	"fmt"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const maxP2PServiceDefault = 1

// BuildServices returns a list of services given the crd.
//
// Creates a single RPC service, likely for use with an Ingress.
//
// Creates 1 p2p service per pod. P2P diverges from traditional web and kubernetes architecture which calls for a single
// p2p service backed by multiple pods.
// Pods may be in various states even with proper readiness probes.
// Therefore, we do not want to confuse or disrupt peer exchange (PEX) within
// tendermint. If using a single p2p service, an outside peer discovering a pod out of sync it could be
// interpreted as byzantine behavior if the peer previously connected to a pod that was in sync through the same
// external address.
func BuildServices(existing []*corev1.Service, crd *cosmosv1.CosmosFullNode) ([]*corev1.Service, error) {
	max := maxP2PServiceDefault
	if v := crd.Spec.Service.MaxP2PExternalAddresses; v != nil {
		max = int(*v)
	}
	maxp2p := lo.Clamp(max, 0, int(crd.Spec.Replicas))
	p2ps := make([]*corev1.Service, maxp2p)

	for i := range lo.Range(maxp2p) {
		ordinal := int32(i)
		var svc corev1.Service
		svc.Name = p2pServiceName(crd, ordinal)
		svc.Namespace = crd.Namespace
		svc.Kind = "Service"
		svc.APIVersion = "v1"

		svc = *kube.FindOrDefaultCopy(existing, &svc)

		svc.Labels = defaultLabels(crd,
			kube.InstanceLabel, instanceName(crd, ordinal),
			kube.ComponentLabel, "p2p",
		)

		var spec corev1.ServiceSpec
		spec.Ports = []corev1.ServicePort{
			{
				Name:       "p2p",
				Protocol:   corev1.ProtocolTCP,
				Port:       p2pPort,
				TargetPort: intstr.FromString("p2p"),
			},
		}
		spec.Selector = map[string]string{kube.InstanceLabel: instanceName(crd, ordinal)}
		spec.Type = corev1.ServiceTypeLoadBalancer
		spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal

		// Preserves existing values such as NodePort values.
		if err := kube.ApplyStrategicMergePatch(&svc.Spec, &spec); err != nil {
			return nil, err
		}

		p2ps[i] = &svc
	}

	rpc, err := rpcService(existing, crd)
	return append(p2ps, rpc), err
}

func rpcService(existing []*corev1.Service, crd *cosmosv1.CosmosFullNode) (*corev1.Service, error) {
	var svc corev1.Service
	svc.Name = rpcServiceName(crd)
	svc.Namespace = crd.Namespace
	svc.Kind = "Service"
	svc.APIVersion = "v1"

	svc = *kube.FindOrDefaultCopy(existing, &svc)

	svc.Labels = defaultLabels(crd,
		kube.ComponentLabel, "rpc",
	)

	var spec corev1.ServiceSpec
	spec.Ports = []corev1.ServicePort{
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
	kube.NormalizeMetadata(&svc.ObjectMeta)

	if v := rpcSpec.ExternalTrafficPolicy; v != nil {
		spec.ExternalTrafficPolicy = *v
	}
	if v := rpcSpec.Type; v != nil {
		spec.Type = *v
	}

	// Preserves existing values such as NodePort values.
	if err := kube.ApplyStrategicMergePatch(&svc.Spec, &spec); err != nil {
		return nil, err
	}

	return &svc, nil
}

func p2pServiceName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return fmt.Sprintf("%s-p2p-%d", appName(crd), ordinal)
}

func rpcServiceName(crd *cosmosv1.CosmosFullNode) string {
	return fmt.Sprintf("%s-rpc", appName(crd))
}
