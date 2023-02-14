package fullnode

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type podDiffer interface {
	Creates() []*corev1.Pod
	Updates() []*corev1.Pod
	Deletes() []*corev1.Pod
}

type PodFilter interface {
	SyncedPods(ctx context.Context, log logr.Logger, candidates []*corev1.Pod) []*corev1.Pod
}

// Client is a controller client. It is a subset of client.Client.
type Client interface {
	client.Reader
	client.Writer

	Scheme() *runtime.Scheme
}

// PodControl reconciles pods for a CosmosFullNode.
type PodControl struct {
	client         Client
	podFilter      PodFilter
	diffFactory    func(ordinalAnnotationKey, revisionLabelKey string, current, want []*corev1.Pod) podDiffer
	computeRollout func(maxUnavail *intstr.IntOrString, desired, ready int) int
}

// NewPodControl returns a valid PodControl.
func NewPodControl(client Client, filter PodFilter) PodControl {
	return PodControl{
		client:    client,
		podFilter: filter,
		diffFactory: func(ordinalAnnotationKey, revisionLabelKey string, current, want []*corev1.Pod) podDiffer {
			return kube.NewOrdinalDiff(ordinalAnnotationKey, revisionLabelKey, current, want)
		},
		computeRollout: kube.ComputeRollout,
	}
}

// Reconcile is the control loop for pods. The bool return value, if true, indicates the controller should requeue
// the request.
func (pc PodControl) Reconcile(ctx context.Context, log logr.Logger, crd *cosmosv1.CosmosFullNode) (bool, kube.ReconcileError) {
	// TODO (nix - 8/9/22) Update crd status.
	// Find any existing pods for this CRD.
	var pods corev1.PodList
	if err := pc.client.List(ctx, &pods,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
		SelectorLabels(crd),
	); err != nil {
		return false, kube.TransientError(fmt.Errorf("list existing pods: %w", err))
	}

	var (
		currentPods = ptrSlice(pods.Items)
		wantPods    = BuildPods(crd)
		diff        = pc.diffFactory(kube.OrdinalAnnotation, kube.RevisionLabel, currentPods, wantPods)
	)

	for _, pod := range diff.Creates() {
		log.Info("Creating pod", "podName", pod.Name)
		if err := ctrl.SetControllerReference(crd, pod, pc.client.Scheme()); err != nil {
			return true, kube.TransientError(fmt.Errorf("set controller reference on pod %q: %w", pod.Name, err))
		}
		if err := pc.client.Create(ctx, pod); kube.IgnoreAlreadyExists(err) != nil {
			return true, kube.TransientError(fmt.Errorf("create pod %q: %w", pod.Name, err))
		}
	}

	for _, pod := range diff.Deletes() {
		log.Info("Deleting pod", "podName", pod.Name)
		if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); kube.IgnoreNotFound(err) != nil {
			return true, kube.TransientError(fmt.Errorf("delete pod %q: %w", pod.Name, err))
		}
	}

	if len(diff.Creates())+len(diff.Deletes()) > 0 {
		// Scaling happens first; then updates. So requeue to handle updates after scaling finished.
		return true, nil
	}

	if len(diff.Updates()) > 0 {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var (
			logger = log.WithName("PodControlRollout")
			// This may be a source of confusion by passing currentPods vs. pods from diff.Updates().
			// This is a leaky abstraction (which may be fixed in the future) because diff.Updates() pods are built
			// from the operator and do not match what's returned by listing pods.
			avail      = pc.podFilter.SyncedPods(cctx, logger, currentPods)
			numUpdates = pc.computeRollout(crd.Spec.RolloutStrategy.MaxUnavailable, int(crd.Spec.Replicas), len(avail))
		)

		for _, pod := range lo.Slice(diff.Updates(), 0, numUpdates) {
			log.Info("Deleting pod for update", "podName", pod.Name)
			// Because we should watch for deletes, we get a re-queued request, detect pod is missing, and re-create it.
			if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				return true, kube.TransientError(fmt.Errorf("update pod %q: %w", pod.Name, err))
			}
		}

		// Signal requeue.
		return true, nil
	}

	// Finished, pod state matches CRD.
	return false, nil
}
