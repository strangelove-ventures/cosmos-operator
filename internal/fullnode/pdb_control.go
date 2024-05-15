package fullnode

import (
	"context"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	policyv1 "k8s.io/api/policy/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceControl creates or updates Services.
type PodDisruptionBudgetControl struct {
	client Client
}

func NewPodDisruptionBudgetControl(client Client) PodDisruptionBudgetControl {
	return PodDisruptionBudgetControl{
		client: client,
	}
}

// Reconcile creates or updates PodDisruptionBudget
func (pdbc PodDisruptionBudgetControl) Reconcile(ctx context.Context, log kube.Logger, crd *cosmosv1.CosmosFullNode) kube.ReconcileError {
	var pdbs policyv1.PodDisruptionBudgetList
	if err := pdbc.client.List(ctx, &pdbs,
		client.InNamespace(crd.Namespace),
		client.MatchingLabels{
			kube.ControllerLabel: "cosmos-operator",
			kube.NameLabel:       appName(crd),
		},
	); err != nil {
		return kube.TransientError(fmt.Errorf("list existing pdb: %w", err))
	}

	current := ptrSlice(pdbs.Items)
	want := BuildPodDisruptionBudget(crd)
	diffed := diff.New(current, want)

	for _, pdb := range diffed.Creates() {
		log.Info("Creating pdb", "name", pdb.Name)
		if err := ctrl.SetControllerReference(crd, pdb, pdbc.client.Scheme()); err != nil {
			return kube.TransientError(fmt.Errorf("set controller reference on pdb %q: %w", pdb.Name, err))
		}
		// CreateOrUpdate (vs. only create) fixes a bug with current deployments where updating would remove the owner reference.
		// This ensures we update the service with the owner reference.
		if err := kube.CreateOrUpdate(ctx, pdbc.client, pdb); err != nil {
			return kube.TransientError(fmt.Errorf("create pdb %q: %w", pdb.Name, err))
		}
	}

	for _, pdb := range diffed.Updates() {
		log.Info("Updating pdb", "name", pdb.Name)
		if err := pdbc.client.Update(ctx, pdb); err != nil {
			return kube.TransientError(fmt.Errorf("update pdb %q: %w", pdb.Name, err))
		}
	}

	return nil
}
