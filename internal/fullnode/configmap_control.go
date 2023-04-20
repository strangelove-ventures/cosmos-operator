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

// ConfigMapControl creates or updates configmaps.
type ConfigMapControl struct {
	build  func(*cosmosv1.CosmosFullNode, Peers) ([]diff.Resource[*corev1.ConfigMap], error)
	client Client
}

// NewConfigMapControl returns a valid ConfigMapControl.
func NewConfigMapControl(client Client) ConfigMapControl {
	return ConfigMapControl{
		build:  BuildConfigMaps,
		client: client,
	}
}

type ConfigChecksums map[client.ObjectKey]string

// Reconcile creates or updates configmaps containing items that are mounted into pods as files.
// The ConfigMap is never deleted unless the CRD itself is deleted.
func (cmc ConfigMapControl) Reconcile(ctx context.Context, log kube.Logger, crd *cosmosv1.CosmosFullNode, peers Peers) (ConfigChecksums, kube.ReconcileError) {
	var cms corev1.ConfigMapList
	if err := cmc.client.List(ctx, &cms,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return nil, kube.TransientError(fmt.Errorf("list existing configmaps: %w", err))
	}

	current := ptrSlice(cms.Items)

	want, err := cmc.build(crd, peers)
	if err != nil {
		return nil, kube.UnrecoverableError(err)
	}

	diffed := diff.New(current, want)

	for _, cm := range diffed.Creates() {
		log.Info("Creating configmap", "configmapName", cm.Name)
		if err := ctrl.SetControllerReference(crd, cm, cmc.client.Scheme()); err != nil {
			return nil, kube.TransientError(fmt.Errorf("set controller reference on configmap %s: %w", cm.Name, err))
		}
		// CreateOrUpdate (vs. only create) fixes a bug with current deployments where updating would remove the owner reference.
		// This ensures we update the service with the owner reference.
		if err := kube.CreateOrUpdate(ctx, cmc.client, cm); err != nil {
			return nil, kube.TransientError(fmt.Errorf("create configmap configmap %s: %w", cm.Name, err))
		}
	}

	for _, cm := range diffed.Deletes() {
		log.Info("Deleting configmap", "configmapName", cm.Name)
		if err := cmc.client.Delete(ctx, cm); err != nil {
			return nil, kube.TransientError(fmt.Errorf("delete configmap %s: %w", cm.Name, err))
		}
	}

	for _, cm := range diffed.Updates() {
		log.Info("Updating configmap", "configmapName", cm.Name)
		if err := cmc.client.Update(ctx, cm); err != nil {
			return nil, kube.TransientError(fmt.Errorf("update configmap %s: %w", cm.Name, err))
		}
	}

	cksums := make(ConfigChecksums)
	for _, cm := range want {
		cksums[client.ObjectKeyFromObject(cm.Object())] = cm.Revision()
	}
	return cksums, nil
}
