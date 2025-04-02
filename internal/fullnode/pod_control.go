package fullnode

import (
	"context"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client is a controller client. It is a subset of client.Client.
type Client interface {
	client.Reader
	client.Writer

	Scheme() *runtime.Scheme
}

type CacheInvalidator interface {
	Invalidate(controller client.ObjectKey, pods []string)
}

// PodControl reconciles pods for a CosmosFullNode.
type PodControl struct {
	client           Client
	cacheInvalidator CacheInvalidator
	computeRollout   func(maxUnavail *intstr.IntOrString, desired, ready int) int
}

// NewPodControl returns a valid PodControl.
func NewPodControl(client Client, cacheInvalidator CacheInvalidator) PodControl {
	return PodControl{
		client:           client,
		cacheInvalidator: cacheInvalidator,
		computeRollout:   kube.ComputeRollout,
	}
}

// Reconcile is the control loop for pods. The bool return value, if true, indicates the controller should requeue
// the request.
func (pc PodControl) Reconcile(
	ctx context.Context,
	reporter kube.Reporter,
	crd *cosmosv1.CosmosFullNode,
	cksums ConfigChecksums,
	syncInfo map[string]*cosmosv1.SyncInfoPodStatus,
) (bool, kube.ReconcileError) {
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

	// Group additional pods by their main pod (replica)
	additionalPodsByReplica := make(map[string][]*corev1.Pod)
	for _, pod := range pods.Items {
		_, isAdditional := pod.Labels[kube.BelongsToLabel]
		if isAdditional && pod.Labels[kube.InstanceLabel] != "" {
			replicaName := pod.Labels[kube.InstanceLabel]
			additionalPodsByReplica[replicaName] = append(additionalPodsByReplica[replicaName], pod.DeepCopy())
		}
	}

	for _, pod := range diffed.Creates() {
		reporter.Info("Creating pod", "name", pod.Name)
		if err := ctrl.SetControllerReference(crd, pod, pc.client.Scheme()); err != nil {
			return true, kube.TransientError(fmt.Errorf("set controller reference on pod %q: %w", pod.Name, err))
		}
		if err := pc.client.Create(ctx, pod); kube.IgnoreAlreadyExists(err) != nil {
			return true, kube.TransientError(fmt.Errorf("create pod %q: %w", pod.Name, err))
		}
	}

	var invalidateCache []string

	defer func() {
		if pc.cacheInvalidator == nil {
			return
		}
		if len(invalidateCache) > 0 {
			pc.cacheInvalidator.Invalidate(client.ObjectKeyFromObject(crd), invalidateCache)
		}
	}()

	// Handle main pod deletions and their associated additional pods
	for _, pod := range diffed.Deletes() {
		_, isAdditional := pod.Labels[kube.BelongsToLabel]

		reporter.Info("Deleting pod", "name", pod.Name, "isMainPod", !isAdditional)

		if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); kube.IgnoreNotFound(err) != nil {
			return true, kube.TransientError(fmt.Errorf("delete pod %q: %w", pod.Name, err))
		}

		delete(syncInfo, pod.Name)
		invalidateCache = append(invalidateCache, pod.Name)
	}

	if len(diffed.Creates())+len(diffed.Deletes()) > 0 {
		// Scaling happens first; then updates. So requeue to handle updates after scaling finished.
		return true, nil
	}

	diffedUpdates := diffed.Updates()
	if len(diffedUpdates) > 0 {
		var (
			updatedPods                      = 0
			rpcReachablePods                 = 0
			inSyncPods                       = 0
			mainPodsToUpdate                 = []*corev1.Pod{}
			additionalPodsToUpdate           = make(map[string][]*corev1.Pod) // Map of main pod name -> additional pods to update
			additionalPodsToUpdateForVersion = make(map[string][]*corev1.Pod) // Map of main pod name -> additional pods to update
			otherUpdates                     = []*corev1.Pod{}                // updates that don't need version upgrade, will be handled after version upgrade if rollout has room to stay above maxUnavailable
			otherUpdatesVersionMismatch      = []*corev1.Pod{}                // updates that need version upgrade and RPC is reachable, will be handled if rollout has room to stay above maxUnavailable
		)

		// Group updates by main pods and additional pods
		for _, update := range diffedUpdates {
			_, isAdditional := update.Labels[kube.BelongsToLabel]

			if isAdditional {
				if replicaName := update.Labels[kube.InstanceLabel]; replicaName != "" {
					additionalPodsToUpdate[replicaName] = append(additionalPodsToUpdate[replicaName], update)
				}
			} else {
				mainPodsToUpdate = append(mainPodsToUpdate, update)
			}
		}

		totalMainPodsToUpdate := len(mainPodsToUpdate)

		// Process main pods and track sync status
		for _, existing := range pods.Items {
			_, isAdditional := existing.Labels[kube.BelongsToLabel]
			if isAdditional || existing.DeletionTimestamp != nil {
				// Skip additional pods and pods being deleted
				continue
			}

			podName := existing.Name

			// Check pod sync status for rollout calculation (only for main pods)
			var rpcReachable bool
			if ps, ok := syncInfo[podName]; ok {
				if ps.InSync != nil && *ps.InSync {
					inSyncPods++
				}
				rpcReachable = ps.Error == nil
				if rpcReachable {
					rpcReachablePods++
				}
			}

			// Find if this pod needs an update
			for _, update := range mainPodsToUpdate {
				if podName == update.Name {
					if existing.Spec.Containers[0].Image != update.Spec.Containers[0].Image {
						// awaiting version upgrade
						if !rpcReachable {
							updatedPods++
							fmt.Printf("Deleting unreachable main pod for version upgrade, name: %s\n", podName) // TODO: remove

							// Delete the main pod
							if err := pc.client.Delete(ctx, update, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
								return true, kube.TransientError(fmt.Errorf("upgrade pod version %q: %w", podName, err))
							}

							syncInfo[podName].InSync = nil
							syncInfo[podName].Error = ptr("version upgrade in progress")
							invalidateCache = append(invalidateCache, podName)

							// Delete all associated additional pods
							for _, additionalPod := range additionalPodsToUpdate[podName] {
								if err := pc.client.Delete(ctx, additionalPod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
									return true, kube.TransientError(fmt.Errorf("upgrade additional pod version %q: %w", additionalPod.Name, err))
								}
							}
							delete(additionalPodsToUpdate, podName)
						} else {
							// RPC is reachable but image needs to update
							otherUpdatesVersionMismatch = append(otherUpdatesVersionMismatch, update)
							additionalPodsToUpdateForVersion[podName] = additionalPodsToUpdate[podName]
							delete(additionalPodsToUpdate, podName)
						}
					} else {
						// Other update
						otherUpdates = append(otherUpdates, update)
					}
					break
				}
			}
		}

		// If we don't have any pods in sync, we are down anyways, so we can use the number of RPC reachable pods for computing the rollout,
		// with the goal of recovering the pods as quickly as possible.
		ready := inSyncPods
		if ready == 0 {
			ready = rpcReachablePods
		}

		// Delete additional pods that need to be updated for non-version reasons
		for _, pods := range additionalPodsToUpdate {
			for _, pod := range pods {
				podName := pod.Name
				reporter.Info("Deleting additional pod for update", "name", podName)

				// Delete the additional pod
				if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
					return true, kube.TransientError(fmt.Errorf("update additional pod %q: %w", podName, err))
				}

				invalidateCache = append(invalidateCache, podName)
			}
		}

		numUpdates := pc.computeRollout(crd.Spec.RolloutStrategy.MaxUnavailable, int(crd.Spec.Replicas), ready)

		if updatedPods == totalMainPodsToUpdate {
			// All main pods are updated.
			return false, nil
		}

		if updatedPods >= numUpdates {
			// Signal requeue.
			return true, nil
		}

		// process upgrade updates first
		for _, pod := range otherUpdatesVersionMismatch {
			podName := pod.Name
			fmt.Printf("Deleting reachable main pod for version upgrade after numupdates, name: %s\n", podName) // TODO: remove
			reporter.Info("Deleting pod for update", "name", podName)
			// Because we should watch for deletes, we get a re-queued request, detect pod is missing, and re-create it.
			if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				return true, kube.TransientError(fmt.Errorf("update pod %q: %w", podName, err))
			}
			syncInfo[podName].InSync = nil
			syncInfo[podName].Error = ptr("update in progress")
			invalidateCache = append(invalidateCache, podName)

			// Delete all associated additional pods
			for _, additionalPod := range additionalPodsToUpdateForVersion[podName] {
				reporter.Info("Deleting additional pod for version upgrade", "name", additionalPod.Name, "mainPod", podName)
				if err := pc.client.Delete(ctx, additionalPod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
					return true, kube.TransientError(fmt.Errorf("upgrade additional pod version %q: %w", additionalPod.Name, err))
				}
			}

			updatedPods++
			if updatedPods >= numUpdates {
				// done for this round
				break
			}
		}

		if updatedPods < numUpdates {
			// process other non-upgrade updates
			for _, pod := range otherUpdates {
				podName := pod.Name
				reporter.Info("Deleting pod for update", "name", podName)
				// Because we should watch for deletes, we get a re-queued request, detect pod is missing, and re-create it.
				if err := pc.client.Delete(ctx, pod, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
					return true, kube.TransientError(fmt.Errorf("update pod %q: %w", podName, err))
				}
				syncInfo[podName].InSync = nil
				syncInfo[podName].Error = ptr("update in progress")
				invalidateCache = append(invalidateCache, podName)
				updatedPods++
				if updatedPods >= numUpdates {
					// done for this round
					break
				}
			}
		}

		if updatedPods < totalMainPodsToUpdate {
			// Not all main pods are updated yet.
			return true, nil
		}

		// Finished, all pods updated
		return false, nil
	}

	// Finished, pod state matches CRD.
	return false, nil
}
