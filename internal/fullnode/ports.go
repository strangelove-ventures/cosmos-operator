package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	apiPort     = 1317
	evmRPCPort  = 8545
	evmWsPort   = 8546
	evmPromPort = 6065
	grpcPort    = 9090
	grpcWebPort = 9091
	p2pPort     = 26656
	privvalPort = 1234
	promPort    = 26660
	rosettaPort = 8080
	rpcPort     = 26657
)

func buildPorts(nodeType cosmosv1.FullNodeType, evmChain bool) []corev1.ContainerPort {
	ports := defaultPorts[:]

	if evmChain {
		ports = append(ports, defaultEVMPorts[:]...)
	}

	switch nodeType {
	case cosmosv1.Sentry:
		return append(ports, corev1.ContainerPort{
			Name:          "privval",
			ContainerPort: privvalPort,
			Protocol:      corev1.ProtocolTCP,
		})
	default:
		return ports
	}
}

var defaultPorts = [...]corev1.ContainerPort{
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

var defaultEVMPorts = [...]corev1.ContainerPort{
	{
		Name:          "evm-rpc",
		ContainerPort: evmRPCPort,
		Protocol:      corev1.ProtocolTCP,
	},
	{
		Name:          "evm-ws",
		ContainerPort: evmWsPort,
		Protocol:      corev1.ProtocolTCP,
	},
	{
		Name:          "evm-prom",
		ContainerPort: evmPromPort,
		Protocol:      corev1.ProtocolTCP,
	},
}
