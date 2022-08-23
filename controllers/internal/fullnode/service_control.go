package fullnode

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Purposefully omitting deletes.
type svcDiffer interface {
	Creates() []*corev1.Service
	Updates() []*corev1.Service
}

// ServiceControl creates or updates Services.
type ServiceControl struct {
	client      Client
	diffFactory func(revisionLabelKey string, current, want []*corev1.Service) svcDiffer
}

func NewServiceControl(client Client) ServiceControl {
	return ServiceControl{
		client: client,
		diffFactory: func(revisionLabelKey string, current, want []*corev1.Service) svcDiffer {
			return kube.NewDiff(revisionLabelKey, current, want)
		},
	}
}

// Reconcile creates or updates services.
// Services care never deleted unless the CRD itself is deleted.
func (sc ServiceControl) Reconcile(ctx context.Context, log logr.Logger, crd *cosmosv1.CosmosFullNode) (ServiceResult, kube.ReconcileError) {
	var result ServiceResult
	var svcs corev1.ServiceList
	if err := sc.client.List(ctx, &svcs,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
		SelectorLabels(crd),
	); err != nil {
		return result, kube.TransientError(fmt.Errorf("list existing services: %w", err))
	}

	var (
		currentSvcs = ptrSlice(svcs.Items)
		wantSvcs    = BuildServices(crd)
		diff        = sc.diffFactory(kube.RevisionLabel, currentSvcs, wantSvcs)
	)

	updateP2PInfo(crd, currentSvcs, &result)

	for _, svc := range diff.Creates() {
		log.Info("Creating service", "svcName", svc.Name)
		if err := ctrl.SetControllerReference(crd, svc, sc.client.Scheme()); err != nil {
			return result, kube.TransientError(fmt.Errorf("set controller reference on service %q: %w", svc.Name, err))
		}
		if err := sc.client.Create(ctx, svc); kube.IgnoreAlreadyExists(err) != nil {
			return result, kube.TransientError(fmt.Errorf("create service %q: %w", svc.Name, err))
		}
	}

	for _, svc := range diff.Updates() {
		log.Info("Updating service", "svcName", svc.Name)
		if err := sc.client.Update(ctx, svc); err != nil {
			return result, kube.TransientError(fmt.Errorf("update service %q: %w", svc.Name, err))
		}
	}

	return result, nil
}

func updateP2PInfo(crd *cosmosv1.CosmosFullNode, current []*corev1.Service, result *ServiceResult) {
	svc, ok := lo.Find(current, func(svc *corev1.Service) bool { return svc.Name == p2pServiceName(crd) })
	if !ok {
		return
	}
	ingress := svc.Status.LoadBalancer.Ingress
	if len(ingress) == 0 {
		return
	}
	result.P2PIPAddress = ingress[0].IP
	result.P2PHostname = ingress[0].Hostname
}

// ServiceResult contains data about Services after reconciliation.
type ServiceResult struct {
	// The p2p IP address from a service. Mutually exclusive with P2PHostname.
	P2PIPAddress string
	// The p2p hostname from a service. Mutually exclusive with P2PIPAddress.
	P2PHostname string
}

// P2PExternalAddress returns the IP or hostname for peers to connect through the public internet.
func (result ServiceResult) P2PExternalAddress() string {
	var host string
	switch {
	case result.P2PIPAddress != "":
		host = result.P2PIPAddress
	case result.P2PHostname != "":
		host = result.P2PHostname
	default:
		return ""
	}
	port := strconv.Itoa(p2pPort)
	return net.JoinHostPort(host, port)
}
