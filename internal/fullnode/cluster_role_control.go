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

// ClusterRoleControl creates or updates ClusterRoles.
type ClusterRoleControl struct {
	client Client
}

func NewClusterRoleControl(client Client) ClusterRoleControl {
	return ClusterRoleControl{
		client: client,
	}
}

// Reconcile creates or updates cluster roles.
func (sc ClusterRoleControl) Reconcile(ctx context.Context, log kube.Logger, crd *cosmosv1.CosmosFullNode) kube.ReconcileError {
	var crs rbacv1.ClusterRoleList
	if err := sc.client.List(ctx, &crs,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return kube.TransientError(fmt.Errorf("list existing cluster roles: %w", err))
	}

	current := ptrSlice(crs.Items)
	want := BuildClusterRoles(crd)
	diffed := diff.New(current, want)

	for _, cr := range diffed.Creates() {
		log.Info("Creating cluster role", "clusterRoleName", cr.Name)
		if err := ctrl.SetControllerReference(crd, cr, sc.client.Scheme()); err != nil {
			return kube.TransientError(fmt.Errorf("set controller reference on cluster role %q: %w", cr.Name, err))
		}
		// CreateOrUpdate (vs. only create) fixes a bug with current deployments where updating would remove the owner reference.
		// This ensures we update the service with the owner reference.
		if err := kube.CreateOrUpdate(ctx, sc.client, cr); err != nil {
			return kube.TransientError(fmt.Errorf("create cluster role %q: %w", cr.Name, err))
		}
	}

	for _, cr := range diffed.Updates() {
		log.Info("Updating cluster role", "clusterRoleName", cr.Name)
		if err := sc.client.Update(ctx, cr); err != nil {
			return kube.TransientError(fmt.Errorf("update cluster role %q: %w", cr.Name, err))
		}
	}

	return nil
}
