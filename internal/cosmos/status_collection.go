package cosmos

import (
	"errors"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// StatusItem is a pod paired with its CometBFT status.
type StatusItem struct {
	Pod    *corev1.Pod
	Status CometStatus
	Ts     time.Time
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
func (status StatusItem) Timestamp() time.Time { return status.Ts }

// StatusCollection is a list of pods and CometBFT status associated with the pod.
type StatusCollection []StatusItem

// UpsertPod updates the pod in the collection or adds an item to the collection if it does not exist.
// All operations are performed in-place.
func UpsertPod(coll *StatusCollection, pod *corev1.Pod) {
	if *coll == nil {
		*coll = make(StatusCollection, 0)
	}
	for i, p := range *coll {
		if p.GetPod().UID == pod.UID {
			item := (*coll)[i]
			(*coll)[i] = StatusItem{Pod: pod, Err: item.Err, Status: item.Status}
			return
		}
	}
	*coll = append(*coll, StatusItem{Pod: pod, Err: errors.New("missing status")})
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

// SyncedPods returns the pods that are caught up with the chain tip.
func (coll StatusCollection) SyncedPods() []*corev1.Pod {
	var pods []*corev1.Pod
	for _, status := range coll {
		if status.Err != nil {
			continue
		}
		if status.Status.Result.SyncInfo.CatchingUp {
			continue
		}
		pods = append(pods, status.Pod)
	}
	return pods
}
