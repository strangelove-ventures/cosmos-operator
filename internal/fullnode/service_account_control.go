package fullnode

import (
	"context"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceControl creates or updates Services.
type ServiceAccountControl struct {
	client Client
}

func NewServiceAccountControl(client Client) ServiceAccountControl {
	return ServiceAccountControl{
		client: client,
	}
}

// Reconcile creates or updates service accounts.
func (sc ServiceAccountControl) Reconcile(ctx context.Context, log kube.Logger, crd *cosmosv1.CosmosFullNode) kube.ReconcileError {
	var sas corev1.ServiceAccountList
	if err := sc.client.List(ctx, &sas,
		client.InNamespace(crd.Namespace),
		client.MatchingLabels{
			kube.ControllerLabel: "cosmos-operator",
			kube.ComponentLabel:  "vc",
			kube.NameLabel:       appName(crd),
		},
	); err != nil {
		return kube.TransientError(fmt.Errorf("list existing service accounts: %w", err))
	}

	current := ptrSlice(sas.Items)
	want := BuildServiceAccounts(crd)
	diffed := diff.New(current, want)

	for _, sa := range diffed.Creates() {
		log.Info("Creating service account", "name", sa.Name)
		if err := ctrl.SetControllerReference(crd, sa, sc.client.Scheme()); err != nil {
			return kube.TransientError(fmt.Errorf("set controller reference on service account %q: %w", sa.Name, err))
		}
		// CreateOrUpdate (vs. only create) fixes a bug with current deployments where updating would remove the owner reference.
		// This ensures we update the service with the owner reference.
		if err := kube.CreateOrUpdate(ctx, sc.client, sa); err != nil {
			return kube.TransientError(fmt.Errorf("create service account %q: %w", sa.Name, err))
		}
	}

	for _, sa := range diffed.Updates() {
		log.Info("Updating service account", "name", sa.Name)
		if err := sc.client.Update(ctx, sa); err != nil {
			return kube.TransientError(fmt.Errorf("update service account %q: %w", sa.Name, err))
		}
	}

	return nil
}
