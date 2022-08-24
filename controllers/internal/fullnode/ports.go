package fullnode

import corev1 "k8s.io/api/core/v1"

const (
	apiPort     = 1317
	grpcPort    = 9090
	grpcWebPort = 9091
	p2pPort     = 26656
	promPort    = 26660
	rosettaPort = 8080
	rpcPort     = 26657
)

var fullNodePorts = []corev1.ContainerPort{
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
		ContainerPort: p2pPort,
	},
	{
		Name:          "rpc",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: rpcPort,
	},
	{
		Name:          "grpc-web",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: grpcWebPort,
	},
}
