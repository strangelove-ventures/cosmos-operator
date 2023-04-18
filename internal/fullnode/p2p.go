package fullnode

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ExternalAddresses keys are instance names and values are public IPs or hostnames.
type ExternalAddresses map[string]string

// Incomplete returns true if any instances do not have a public IP or hostname.
func (addrs ExternalAddresses) Incomplete() bool {
	for _, v := range addrs {
		if v == "" {
			return true
		}
	}
	return false
}

// CollectExternalP2P collects external addresses from p2p services.
func CollectExternalP2P(ctx context.Context, crd *cosmosv1.CosmosFullNode, c Lister) (ExternalAddresses, kube.ReconcileError) {
	var svcs corev1.ServiceList
	if err := c.List(ctx, &svcs,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name, ".spec.type": string(corev1.ServiceTypeLoadBalancer)},
		client.MatchingLabels{kube.ComponentLabel: "p2p"},
	); err != nil {
		return nil, kube.TransientError(fmt.Errorf("list existing services: %w", err))
	}

	var (
		port = strconv.Itoa(p2pPort)
		m    = make(ExternalAddresses)
	)
	for _, svc := range svcs.Items {
		ingress := svc.Status.LoadBalancer.Ingress
		if len(ingress) == 0 {
			m[svc.Labels[kube.InstanceLabel]] = ""
			continue
		}
		lb := ingress[0]
		host := lo.Ternary(lb.IP != "", lb.IP, lb.Hostname)
		if host != "" {
			m[svc.Labels[kube.InstanceLabel]] = net.JoinHostPort(host, port)
		}
	}
	return m, nil
}
