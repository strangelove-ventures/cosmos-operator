package fullnode

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	kube2 "github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type configmapDiffer interface {
	Creates() []*corev1.ConfigMap
	Updates() []*corev1.ConfigMap
	Deletes() []*corev1.ConfigMap
}

// ConfigMapControl creates or updates configmaps.
type ConfigMapControl struct {
	build       func(*cosmosv1.CosmosFullNode, ExternalAddresses) ([]*corev1.ConfigMap, error)
	client      Client
	diffFactory func(revisionLabelKey string, current, want []*corev1.ConfigMap) configmapDiffer
}

// NewConfigMapControl returns a valid ConfigMapControl.
func NewConfigMapControl(client Client) ConfigMapControl {
	return ConfigMapControl{
		build:  BuildConfigMaps,
		client: client,
		diffFactory: func(revisionLabelKey string, current, want []*corev1.ConfigMap) configmapDiffer {
			return kube2.NewDiff(revisionLabelKey, current, want)
		},
	}
}

// Reconcile creates or updates configmaps containing items that are mounted into pods as files.
// The ConfigMap is never deleted unless the CRD itself is deleted.
func (cmc ConfigMapControl) Reconcile(ctx context.Context, log logr.Logger, crd *cosmosv1.CosmosFullNode, p2p ExternalAddresses) kube2.ReconcileError {
	want, err := cmc.build(crd, p2p)
	if err != nil {
		return kube2.UnrecoverableError(err)
	}

	var cms corev1.ConfigMapList
	if err := cmc.client.List(ctx, &cms,
		client.InNamespace(crd.Namespace),
		SelectorLabels(crd),
	); err != nil {
		return kube2.TransientError(fmt.Errorf("list existing configmaps: %w", err))
	}

	var (
		current = ptrSlice(cms.Items)
		diff    = cmc.diffFactory(kube2.RevisionLabel, current, want)
	)

	for _, cm := range diff.Creates() {
		log.Info("Creating configmap", "configmapName", cm.Name)
		if err := ctrl.SetControllerReference(crd, cm, cmc.client.Scheme()); err != nil {
			return kube2.TransientError(fmt.Errorf("set controller reference on configmap %q: %w", cm.Name, err))
		}
		if err := cmc.client.Create(ctx, cm); kube2.IgnoreAlreadyExists(err) != nil {
			return kube2.TransientError(fmt.Errorf("create service %q: %w", cm.Name, err))
		}
	}

	for _, cm := range diff.Deletes() {
		log.Info("Deleting configmap", "configmapName", cm.Name)
		if err := cmc.client.Delete(ctx, cm); err != nil {
			return kube2.TransientError(fmt.Errorf("delete configmap %q: %w", cm.Name, err))
		}
	}

	for _, cm := range diff.Updates() {
		log.Info("Updating configmap", "configmapName", cm.Name)
		if err := cmc.client.Update(ctx, cm); err != nil {
			return kube2.TransientError(fmt.Errorf("update configmap %q: %w", cm.Name, err))
		}
	}

	return nil
}
