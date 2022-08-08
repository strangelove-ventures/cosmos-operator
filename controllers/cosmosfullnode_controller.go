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
	"time"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
	finishResult  ctrl.Result
	requeueResult = ctrl.Result{RequeueAfter: 3 * time.Second}
)

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/finalizers,verbs=update
// Generate RBAC roles to watch and update Pods
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;watch;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
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
		return finishResult, client.IgnoreNotFound(err)
	}

	// Find any existing pods for this CRD.
	var pods corev1.PodList
	if err := r.List(ctx, &pods,
		client.InNamespace(req.Namespace),
		client.MatchingFields{controllerOwnerField: req.Name},
		fullnode.SelectorLabels(&crd),
	); err != nil {
		return requeueResult, fmt.Errorf("list existing pods: %w", err)
	}

	if len(pods.Items) > 0 {
		logger.V(2).Info("Found existing pods", "numPods", len(pods.Items))
	} else {
		logger.V(2).Info("Did not find any existing pods")
	}

	var (
		currentPods = ptrSlice(pods.Items)
		wantPods    = fullnode.PodState(&crd)
		podDiff     = kube.NewDiff(fullnode.OrdinalAnnotation, currentPods, wantPods)
	)

	for _, pod := range podDiff.Creates() {
		logger.Info("Creating pod", "podName", pod.Name)
		// TODO (nix - 7/25/22) This step is easy to forget. Perhaps abstract it somewhere.
		if err := ctrl.SetControllerReference(&crd, pod, r.Scheme); err != nil {
			return requeueResult, fmt.Errorf("set controller reference on pod %q: %w", pod.Name, err)
		}
		if err := r.Create(ctx, pod); err != nil {
			return requeueResult, fmt.Errorf("create pod %q: %w", pod.Name, err)
		}
	}

	for _, pod := range podDiff.Deletes() {
		logger.Info("Deleting pod", "podName", pod.Name)
		if err := r.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
			return requeueResult, fmt.Errorf("delete pod %q: %w", pod.Name, err)
		}
	}

	if len(podDiff.Updates()) == 0 {
		return finishResult, nil
	}

	var (
		avail      = kube.AvailablePods(currentPods, 3*time.Second, time.Now())
		numUpdates = kube.ComputeRollout(crd.Spec.RolloutStrategy.MaxUnavailable, int(crd.Spec.Replicas), len(avail))
	)

	logger.Info("Update pod stats", "replicas", crd.Spec.Replicas, "numReady", len(avail), "numUpdates", numUpdates)

	for _, pod := range lo.Slice(podDiff.Updates(), 0, numUpdates) {
		logger.Info("Updating pod", "podName", pod.Name)
		// Because we watch for deletes, we get a re-queued request, detect pod is missing, and re-create it.
		if err := r.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
			return finishResult, fmt.Errorf("update pod %q: %w", pod.Name, err)
		}
	}

	logger.Info("Requeue for pod rollout")
	return requeueResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CosmosFullNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Pod{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.Pod]("CosmosFullNode"),
	)
	if err != nil {
		return fmt.Errorf("index field %s: %w", controllerOwnerField, err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1.CosmosFullNode{}).
		// Watch some pod events to queue requests for pods owned by CosmosFullNode controller.
		Watches(
			&source.Kind{Type: &corev1.Pod{}},
			&handler.EnqueueRequestForOwner{OwnerType: &cosmosv1.CosmosFullNode{}, IsController: true},
			builder.WithPredicates(&predicate.Funcs{
				DeleteFunc: func(_ event.DeleteEvent) bool { return true },
			},
			),
		).
		Complete(r)
}
