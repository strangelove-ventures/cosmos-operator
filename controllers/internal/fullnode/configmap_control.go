package fullnode

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigMapControl creates or updates configmaps.
type ConfigMapControl struct {
	//build  func(tendermint cosmosv1.CosmosTendermintConfig, app cosmosv1.CosmosAppConfig) (corev1.ConfigMap, error)
	//build
	client Client
}

// NewConfigMapControl returns a valid ConfigMapControl.
func NewConfigMapControl(client Client) ConfigMapControl {
	return ConfigMapControl{
		//build:  BuildConfigMap,
		client: client,
	}
}

// Reconcile creates or updates configmaps containing items that are mounted into pods as files.
func (cmc ConfigMapControl) Reconcile(ctx context.Context, log logr.Logger, crd *cosmosv1.CosmosFullNode) (bool, kube.ReconcileError) {
	want, err := BuildConfigMap(crd)
	if err != nil {
		return false, kube.UnrecoverableError(err)
	}

	var (
		current corev1.ConfigMap
		key     = client.ObjectKey{Namespace: crd.Namespace, Name: configMapName(crd)}
	)
	err = cmc.client.Get(ctx, key, &current)
	if kube.IsNotFound(err) {
		log.Info("Creating ConfigMap", "configMapName", key.Name)
		return cmc.create(ctx, crd, want)
	}
	if err != nil {
		return false, kube.TransientError(err)
	}

	if !reflect.DeepEqual(current.Labels, want.Labels) ||
		!reflect.DeepEqual(current.Data, want.Data) ||
		!reflect.DeepEqual(current.BinaryData, want.BinaryData) {
		if err := cmc.client.Update(ctx, &want); err != nil {
			log.Info("Updating ConfigMap", "configMapName", key.Name)
			return false, kube.TransientError(err)
		}
	}

	return false, nil
}

func (cmc ConfigMapControl) create(ctx context.Context, crd *cosmosv1.CosmosFullNode, cm corev1.ConfigMap) (bool, kube.ReconcileError) {
	if err := ctrl.SetControllerReference(crd, &cm, cmc.client.Scheme()); err != nil {
		return true, kube.TransientError(fmt.Errorf("set controller reference on configmap %q: %w", cm.Name, err))
	}
	if err := cmc.client.Create(ctx, &cm); kube.IgnoreAlreadyExists(err) != nil {
		return false, kube.TransientError(err)
	}
	return false, nil
}
