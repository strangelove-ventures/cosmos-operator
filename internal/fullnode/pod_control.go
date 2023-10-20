package fullnode

import (
	"context"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodFilter interface {
	PodsWithStatus(ctx context.Context, crd *cosmosv1.CosmosFullNode) []cosmos.PodStatus
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
	computeRollout func(maxUnavail *intstr.IntOrString, desired, ready int) int
}

// NewPodControl returns a valid PodControl.
func NewPodControl(client Client, filter PodFilter) PodControl {
	return PodControl{
		client:         client,
		podFilter:      filter,
		computeRollout: kube.ComputeRollout,
	}
}

// Reconcile is the control loop for pods. The bool return value, if true, indicates the controller should requeue
// the request.
func (pc PodControl) Reconcile(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode, cksums ConfigChecksums) (bool, kube.ReconcileError) {
	var pods corev1.PodList
	if err := pc.client.List(ctx, &pods,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return false, kube.TransientError(fmt.Errorf("list existing pods: %w", err))
	}

	wantPods, err := BuildPods(crd, cksums)
	if err != nil {
		return false, kube.UnrecoverableError(fmt.Errorf("build pods: %w", err))
	}
	diffed := diff.New(ptrSlice(pods.Items), wantPods)

	for _, pod := range diffed.Creates() {
		reporter.Info("Creating pod", "pod", pod.Name)
		if err := ctrl.SetControllerReference(crd, pod, pc.client.Scheme()); err != nil {
			return true, kube.TransientError(fmt.Errorf("set controller reference on pod %q: %w", pod.Name, err))
		}
		if err := pc.client.Create(ctx, pod); kube.IgnoreAlreadyExists(err) != nil {
			return true, kube.TransientError(fmt.Errorf("create pod %q: %w", pod.Name, err))
		}
	}

	for _, pod := range diffed.Deletes() {
		reporter.Info("Deleting pod", "pod", pod.Name)
		if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); kube.IgnoreNotFound(err) != nil {
			return true, kube.TransientError(fmt.Errorf("delete pod %q: %w", pod.Name, err))
		}
	}

	if len(diffed.Creates())+len(diffed.Deletes()) > 0 {
		// Scaling happens first; then updates. So requeue to handle updates after scaling finished.
		return true, nil
	}

	diffedUpdates := diffed.Updates()
	if len(diffedUpdates) > 0 {
		var (
			podsWithStatus   = pc.podFilter.PodsWithStatus(ctx, crd)
			upgradePods      = make(map[string]bool)
			otherUpdates     = make(map[string]*corev1.Pod)
			rpcReachablePods = make(map[string]bool)
			inSyncPods       = make(map[string]bool)
			deletedPods      = make(map[string]bool)
		)

		for _, update := range diffedUpdates {
			for _, ps := range podsWithStatus {
				if ps.Pod.Name == update.Name {
					if ps.Synced {
						inSyncPods[ps.Pod.Name] = true
					}
					if ps.RPCReachable {
						rpcReachablePods[ps.Pod.Name] = true
					}
					if ps.AwaitingUpgrade {
						if !ps.RPCReachable {
							upgradePods[ps.Pod.Name] = true
							reporter.Info("Deleting pod for version upgrade", "podName", ps.Pod.Name)
							// Because we should watch for deletes, we get a re-queued request, detect pod is missing, and re-create it.
							if err := pc.client.Delete(ctx, ps.Pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
								return true, kube.TransientError(fmt.Errorf("upgrade pod version %q: %w", ps.Pod.Name, err))
							}
							deletedPods[ps.Pod.Name] = true
						} else {
							otherUpdates[ps.Pod.Name] = ps.Pod
						}
					} else {
						otherUpdates[ps.Pod.Name] = ps.Pod
					}
					break
				}
			}
		}

		// If we don't have any pods in sync, we are down anyways, so we can use the number of RPC reachable pods for computing the rollout,
		// with the goal of recovering the pods as quickly as possible.
		ready := len(inSyncPods)
		if ready == 0 {
			ready = len(rpcReachablePods)
		}

		numUpdates := pc.computeRollout(crd.Spec.RolloutStrategy.MaxUnavailable, int(crd.Spec.Replicas), ready)

		updated := len(upgradePods)

		if updated == len(diffedUpdates) {
			// All pods are updated.
			return false, nil
		}

		if updated >= numUpdates {
			// Signal requeue.
			return true, nil
		}

		for podName, pod := range otherUpdates {
			reporter.Info("Deleting pod for update", "podName", podName)
			// Because we should watch for deletes, we get a re-queued request, detect pod is missing, and re-create it.
			if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				return true, kube.TransientError(fmt.Errorf("update pod %q: %w", podName, err))
			}
			deletedPods[podName] = true
			updated++
			if updated >= numUpdates {
				// done for this round
				break
			}
		}

		if len(diffedUpdates) == updated {
			// All pods are updated.
			return false, nil
		}

		// Signal requeue.
		return true, nil
	}

	// Finished, pod state matches CRD.
	return false, nil
}
