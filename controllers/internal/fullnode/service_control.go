package fullnode

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
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

type ServiceReconcileResult struct {
	P2PIP string
}

// Reconcile creates or updates services.
// Services care never deleted unless the CRD itself is deleted.
func (sc ServiceControl) Reconcile(ctx context.Context, log logr.Logger, crd *cosmosv1.CosmosFullNode) (ServiceReconcileResult, kube.ReconcileError) {
	var result ServiceReconcileResult
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
