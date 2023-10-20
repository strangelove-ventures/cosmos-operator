package cosmos

import (
	"errors"
	"strconv"
	"time"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// StatusItem is a pod paired with its CometBFT status.
type StatusItem struct {
	Pod    *corev1.Pod
	Status CometStatus
	TS     time.Time
	Err    error
}

// GetPod returns the pod.
func (status StatusItem) GetPod() *corev1.Pod {
	return status.Pod
}

// GetStatus returns the CometBFT status or an error if the status could not be fetched.
func (status StatusItem) GetStatus() (CometStatus, error) {
	return status.Status, status.Err
}

// Timestamp returns the time when the CometBFT status was fetched.
func (status StatusItem) Timestamp() time.Time { return status.TS }

// StatusCollection is a list of pods and CometBFT status associated with the pod.
type StatusCollection []StatusItem

// Len returns the number of items in the collection. Part of the sort.Interface implementation.
func (coll StatusCollection) Len() int { return len(coll) }

// Less implements sort.Interface.
func (coll StatusCollection) Less(i, j int) bool {
	lhs, _ := strconv.Atoi(coll[i].GetPod().Annotations[kube.OrdinalAnnotation])
	rhs, _ := strconv.Atoi(coll[j].GetPod().Annotations[kube.OrdinalAnnotation])
	return lhs < rhs
}

// Swap implements sort.Interface.
func (coll StatusCollection) Swap(i, j int) {
	coll[i], coll[j] = coll[j], coll[i]
}

// UpsertPod updates the pod in the collection or adds an item to the collection if it does not exist.
// All operations are performed in-place.
func UpsertPod(coll *StatusCollection, pod *corev1.Pod) {
	if *coll == nil {
		*coll = make(StatusCollection, 0)
	}
	for i, p := range *coll {
		if p.GetPod().UID == pod.UID {
			item := (*coll)[i]
			item.Pod = pod
			(*coll)[i] = item
			return
		}
	}
	*coll = append(*coll, StatusItem{Pod: pod, TS: time.Now(), Err: errors.New("missing status")})
}

// IntersectPods removes all pods from the collection that are not in the given list.
func IntersectPods(coll *StatusCollection, pods []corev1.Pod) {
	if *coll == nil {
		*coll = make(StatusCollection, 0)
		return
	}

	set := make(map[types.UID]bool)
	for _, pod := range pods {
		set[pod.UID] = true
	}

	var j int
	for _, item := range *coll {
		if set[item.GetPod().UID] {
			(*coll)[j] = item
			j++
		}
	}
	*coll = (*coll)[:j]
}

// Pods returns all pods.
func (coll StatusCollection) Pods() []*corev1.Pod {
	return lo.Map(coll, func(status StatusItem, _ int) *corev1.Pod { return status.GetPod() })
}

// Synced returns all items that are caught up with the chain tip.
func (coll StatusCollection) Synced() StatusCollection {
	var items []StatusItem
	for _, status := range coll {
		if status.Err != nil {
			continue
		}
		if status.Status.Result.SyncInfo.CatchingUp {
			continue
		}
		items = append(items, status)
	}
	return items
}

// SyncedPods returns the pods that are caught up with the chain tip.
func (coll StatusCollection) SyncedPods() []*corev1.Pod {
	return lo.Map(coll.Synced(), func(status StatusItem, _ int) *corev1.Pod { return status.GetPod() })
}

// PodStatus is the status of a pod.
type PodStatus struct {
	Pod             *corev1.Pod
	RPCReachable    bool
	Synced          bool
	AwaitingUpgrade bool
}

// PodsWithStatus returns all pods with their status.
func (coll StatusCollection) PodsWithStatus(crd *cosmosv1.CosmosFullNode) []PodStatus {
	out := make([]PodStatus, len(coll))
	for i, status := range coll {
		ps := PodStatus{
			Pod: status.GetPod(),
		}
		if crd.Spec.ChainSpec.Versions != nil {
			instanceHeight := uint64(0)
			if height, ok := crd.Status.Height[status.Pod.Name]; ok {
				instanceHeight = height
			}
			var image string
			for _, version := range crd.Spec.ChainSpec.Versions {
				if instanceHeight < version.UpgradeHeight {
					break
				}
				image = version.Image
			}
			if image != "" && status.Pod.Spec.Containers[0].Image != image {
				ps.AwaitingUpgrade = true
			}
		}
		if status.Err == nil {
			ps.RPCReachable = true
			if !status.Status.Result.SyncInfo.CatchingUp {
				ps.Synced = true
			}
		}

		out[i] = ps
	}
	return out
}
