/*
Copyright 2022 Strangelove Ventures LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/fullnode"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	controllerOwnerField = ".metadata.controller"
)

// CosmosFullNodeReconciler reconciles a CosmosFullNode object
type CosmosFullNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var (
	emptyResult ctrl.Result
)

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CosmosFullNode object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *CosmosFullNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the CRD
	var crd cosmosv1.CosmosFullNode
	if err := r.Get(ctx, req.NamespacedName, &crd); err != nil {
		// Ignore not found errors because can't be fixed by an immediate requeue. We'll have to wait for next notification.
		// Also, will get "not found" error if crd is deleted.
		// No need to explicitly delete resources. Kube GC does so automatically because we set the controller reference
		// for each resource.
		return emptyResult, client.IgnoreNotFound(err)
	}

	// Find any existing pods for this CRD.
	var pods corev1.PodList
	if err := r.List(ctx, &pods,
		client.InNamespace(req.Namespace),
		client.MatchingFields{controllerOwnerField: req.Name},
		fullnode.SelectorLabels(&crd),
	); err != nil {
		return emptyResult, fmt.Errorf("list existing pods: %w", err)
	}

	// TODO: do something important besides just log here
	logger.Info("Found existing pods for CRD", "numPods", len(pods.Items))

	builder := fullnode.NewPodBuilder(&crd)
	// TODO: not idempotent
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		pod, err := builder.WithOrdinal(i).WithOwner(r.Scheme).Build()
		if err != nil {
			return emptyResult, err
		}
		err = r.Create(ctx, pod)
		if err != nil {
			return emptyResult, fmt.Errorf("create for %s: pod %s: %w", req.NamespacedName, pod.Name, err)
		}
	}

	return emptyResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CosmosFullNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Pod{},
		controllerOwnerField,
		cosmos.IndexOwner[*corev1.Pod]("CosmosFullNode"),
	)
	if err != nil {
		return fmt.Errorf("index field %s: %w", controllerOwnerField, err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1.CosmosFullNode{}).
		Complete(r)
}
