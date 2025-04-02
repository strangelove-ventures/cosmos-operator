package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	apiPort     = 1317
	grpcPort    = 9090
	grpcWebPort = 9091
	privvalPort = 1234
	promPort    = 26660
	rosettaPort = 8080
	p2pPort     = 26656
	rpcPort     = 26657
)

func buildPorts(crd *cosmosv1.CosmosFullNode) []corev1.ContainerPort {
	switch crd.Spec.Type {
	case cosmosv1.Sentry:
		return append(ports(crd), corev1.ContainerPort{
			Name:          "privval",
			ContainerPort: privvalPort,
			Protocol:      corev1.ProtocolTCP,
		})
	default:
		return ports(crd)
	}
}

func ports(crd *cosmosv1.CosmosFullNode) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "api",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: apiPort,
		},
		{
			Name:          "rosetta",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: rosettaPort,
		},
		{
			Name:          "grpc",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: grpcPort,
		},
		{
			Name:          "prometheus",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: promPort,
		},
		{
			Name:          "p2p",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: crd.Spec.ChainSpec.Comet.P2PPort(),
		},
		{
			Name:          "rpc",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: crd.Spec.ChainSpec.Comet.RPCPort(),
		},
		{
			Name:          "grpc-web",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: grpcWebPort,
		},
	}
}
