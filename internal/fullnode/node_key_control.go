package fullnode

import (
	"context"
	"fmt"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/bharvest-devops/cosmos-operator/internal/diff"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeKeyControl reconciles node keys for a CosmosFullNode. Node keys are saved as secrets and later mounted
// into pods.
type NodeKeyControl struct {
	client Client
}

func NewNodeKeyControl(client Client) NodeKeyControl {
	return NodeKeyControl{
		client: client,
	}
}

// Reconcile is the control loop for node keys. The secrets are never deleted.
func (control NodeKeyControl) Reconcile(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode) kube.ReconcileError {
	var secrets corev1.SecretList
	if err := control.client.List(ctx, &secrets,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return kube.TransientError(fmt.Errorf("list existing node key secrets: %w", err))
	}

	existing := ptrSlice(secrets.Items)
	want, serr := BuildNodeKeySecrets(existing, crd)
	if serr != nil {
		return kube.UnrecoverableError(fmt.Errorf("build node key secrets: %w", serr))
	}

	diffed := diff.New(existing, want)

	for _, secret := range diffed.Creates() {
		reporter.Info("Creating node key secret", "secret", secret.Name)
		if err := ctrl.SetControllerReference(crd, secret, control.client.Scheme()); err != nil {
			return kube.TransientError(fmt.Errorf("set controller reference on node key secret %q: %w", secret.Name, err))
		}
		if err := control.client.Create(ctx, secret); kube.IgnoreAlreadyExists(err) != nil {
			return kube.TransientError(fmt.Errorf("create node key secret %q: %w", secret.Name, err))
		}
	}

	for _, secret := range diffed.Updates() {
		reporter.Info("Updating node key secret", "secret", secret.Name)
		if err := control.client.Update(ctx, secret); err != nil {
			return kube.TransientError(fmt.Errorf("update node key secret %q: %w", secret.Name, err))
		}
	}

	return nil
}
