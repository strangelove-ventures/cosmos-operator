package fullnode

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func BuildServices(crd *cosmosv1.CosmosFullNode) []*corev1.Service {
	max := maxP2PServiceDefault
	if v := crd.Spec.Service.MaxP2PExternalAddresses; v != nil {
		max = int(*v)
	}
	maxp2p := lo.Clamp(max, 0, int(crd.Spec.Replicas))
	p2ps := make([]*corev1.Service, maxp2p)

	for i := range lo.Range(maxp2p) {
		ordinal := int32(i)
		labels := defaultLabels(crd,
			kube.RevisionLabel, serviceRevisionHash(crd),
			kube.InstanceLabel, instanceName(crd, ordinal),
			kube.ComponentLabel, "p2p",
		)
		p2ps[i] = &corev1.Service{
			TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      p2pServiceName(crd, ordinal),
				Namespace: crd.Namespace,
				Labels:    labels,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "p2p",
						Protocol:   corev1.ProtocolTCP,
						Port:       p2pPort,
						TargetPort: intstr.FromString("p2p"),
					},
				},
				Selector:              map[string]string{kube.InstanceLabel: instanceName(crd, ordinal)},
				Type:                  corev1.ServiceTypeLoadBalancer,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			},
		}
	}

	rpc := rpcService(crd)
	return append(p2ps, rpc)
}

func rpcService(crd *cosmosv1.CosmosFullNode) *corev1.Service {
	labels := defaultLabels(crd,
		kube.RevisionLabel, serviceRevisionHash(crd),
		kube.ComponentLabel, "rpc",
	)

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        rpcServiceName(crd),
			Namespace:   crd.Namespace,
			Labels:      labels,
			Annotations: make(map[string]string),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
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
			},
			Selector: map[string]string{kube.NameLabel: appName(crd)},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	spec := crd.Spec.Service.RPCTemplate
	preserveMergeInto(svc.Labels, spec.Metadata.Labels)
	preserveMergeInto(svc.Annotations, spec.Metadata.Annotations)
	kube.NormalizeMetadata(&svc.ObjectMeta)

	if v := spec.ExternalTrafficPolicy; v != nil {
		svc.Spec.ExternalTrafficPolicy = *v
	}
	if v := spec.Type; v != nil {
		svc.Spec.Type = *v
	}

	return svc
}

func p2pServiceName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return fmt.Sprintf("%s-p2p-%d", appName(crd), ordinal)
}

func rpcServiceName(crd *cosmosv1.CosmosFullNode) string {
	return fmt.Sprintf("%s-rpc", appName(crd))
}

// only requires update if the labels change
func serviceRevisionHash(crd *cosmosv1.CosmosFullNode) string {
	h := fnv.New32()

	labels := lo.MapToSlice(defaultLabels(crd), func(v string, k string) string {
		return k + v
	})
	sort.Strings(labels)
	mustWrite(h, strings.Join(labels, ""))
	mustWrite(h, mustMarshalJSON(crd.Spec.Service.RPCTemplate))

	return hex.EncodeToString(h.Sum(nil))
}
