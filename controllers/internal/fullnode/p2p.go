package fullnode

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Lister can list resources, subset of client.Client.
type Lister interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

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

// CollectP2PAddresses collects external addresses from p2p services.
func CollectP2PAddresses(ctx context.Context, crd *cosmosv1.CosmosFullNode, c Lister) (ExternalAddresses, kube.ReconcileError) {
	var svcs corev1.ServiceList
	selector := SelectorLabels(crd)
	selector[kube.ComponentLabel] = "p2p"
	if err := c.List(ctx, &svcs,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
		selector,
	); err != nil {
		return nil, kube.TransientError(fmt.Errorf("list existing services: %w", err))
	}

	m := make(map[string]string)
	for _, svc := range svcs.Items {
		ingress := svc.Status.LoadBalancer.Ingress
		if len(ingress) == 0 {
			m[svc.Labels[kube.InstanceLabel]] = ""
			continue
		}
		lb := ingress[0]
		m[svc.Labels[kube.InstanceLabel]] = lo.Ternary(lb.IP != "", lb.IP, lb.Hostname)
	}
	return m, nil
}
