package fullnode

import (
	"context"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RoleBindingControl creates or updates RoleBindings.
type RoleBindingControl struct {
	client Client
}

func NewRoleBindingControl(client Client) RoleBindingControl {
	return RoleBindingControl{
		client: client,
	}
}

// Reconcile creates or updates role bindings.
func (sc RoleBindingControl) Reconcile(ctx context.Context, log kube.Logger, crd *cosmosv1.CosmosFullNode) kube.ReconcileError {
	var crs rbacv1.RoleBindingList
	if err := sc.client.List(ctx, &crs,
		client.InNamespace(crd.Namespace),
		client.MatchingLabels{
			kube.ControllerLabel: "cosmos-operator",
			kube.ComponentLabel:  "vc",
			kube.NameLabel:       appName(crd),
		},
	); err != nil {
		return kube.TransientError(fmt.Errorf("list existing role bindings: %w", err))
	}

	current := ptrSlice(crs.Items)
	want := BuildRoleBindings(crd)
	diffed := diff.New(current, want)

	for _, cr := range diffed.Creates() {
		log.Info("Creating role binding", "name", cr.Name)
		if err := ctrl.SetControllerReference(crd, cr, sc.client.Scheme()); err != nil {
			return kube.TransientError(fmt.Errorf("set controller reference on role binding %q: %w", cr.Name, err))
		}
		// CreateOrUpdate (vs. only create) fixes a bug with current deployments where updating would remove the owner reference.
		// This ensures we update the service with the owner reference.
		if err := kube.CreateOrUpdate(ctx, sc.client, cr); err != nil {
			return kube.TransientError(fmt.Errorf("create role binding %q: %w", cr.Name, err))
		}
	}

	for _, cr := range diffed.Updates() {
		log.Info("Updating role binding", "name", cr.Name)
		if err := sc.client.Update(ctx, cr); err != nil {
			return kube.TransientError(fmt.Errorf("update role binding %q: %w", cr.Name, err))
		}
	}

	return nil
}
