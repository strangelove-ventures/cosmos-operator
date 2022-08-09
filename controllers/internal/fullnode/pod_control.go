package fullnode

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodControl struct {
	log    logr.Logger
	client client.Client
}

func NewPodControl(logger logr.Logger, client client.Client) PodControl {
	return PodControl{
		log:    logger,
		client: client,
	}
}

func (pc PodControl) Reconcile(ctx context.Context, req ctrl.Request, crd *cosmosv1.CosmosFullNode) kube.ReconcileError {
	// Find any existing pods for this CRD.
	var pods corev1.PodList
	if err := pc.client.List(ctx, &pods,
		client.InNamespace(req.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: req.Name},
		SelectorLabels(crd),
	); err != nil {
		return kube.TransientError(fmt.Errorf("list existing pods: %w", err))
	}

	if len(pods.Items) > 0 {
		pc.log.V(2).Info("Found existing pods", "numPods", len(pods.Items))
	} else {
		pc.log.V(2).Info("Did not find any existing pods")
	}

	var (
		currentPods = ptrSlice(pods.Items)
		wantPods    = PodState(crd)
		diff        = kube.NewDiff(OrdinalAnnotation, currentPods, wantPods)
	)

	for _, pod := range diff.Creates() {
		pc.log.Info("Creating pod", "podName", pod.Name)
		if err := ctrl.SetControllerReference(crd, pod, pc.client.Scheme()); err != nil {
			return kube.TransientError(fmt.Errorf("set controller reference on pod %q: %w", pod.Name, err))
		}
		if err := pc.client.Create(ctx, pod); err != nil {
			return kube.TransientError(fmt.Errorf("create pod %q: %w", pod.Name, err))
		}
	}

	for _, pod := range diff.Deletes() {
		pc.log.Info("Deleting pod", "podName", pod.Name)
		if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
			return kube.TransientError(fmt.Errorf("delete pod %q: %w", pod.Name, err))
		}
	}

	if len(diff.Creates())+len(diff.Deletes()) > 0 {
		// Scaling happens first. Then updates.
		return kube.TransientError(errors.New("scaling in progress"))
	}

	if len(diff.Updates()) > 0 {
		var (
			avail      = kube.AvailablePods(currentPods, 3*time.Second, time.Now())
			numUpdates = kube.ComputeRollout(crd.Spec.RolloutStrategy.MaxUnavailable, int(crd.Spec.Replicas), len(avail))
		)

		for _, pod := range lo.Slice(diff.Updates(), 0, numUpdates) {
			pc.log.Info("Deleting pod for update", "podName", pod.Name)
			// Because we should watch for deletes, we get a re-queued request, detect pod is missing, and re-create it.
			if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				return kube.TransientError(fmt.Errorf("update pod %q: %w", pod.Name, err))
			}
		}

		return kube.TransientError(errors.New("rollout in progress"))
	}

	return nil
}
