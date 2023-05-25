package cosmos

import (
	"errors"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
)

// StatusItem is a pod paired with its CometBFT status.
type StatusItem struct {
	pod    *corev1.Pod
	status CometStatus
	err    error
}

// Pod returns the pod.
func (status StatusItem) Pod() *corev1.Pod {
	return status.pod
}

// Status returns the CometBFT status or an error if the status could not be fetched.
func (status StatusItem) Status() (CometStatus, error) {
	return status.status, status.err
}

// StatusCollection is a list of pods and CometBFT status associated with the pod.
type StatusCollection []StatusItem

// UpsertPod updates the pod in the collection or adds an item to the collection if it does not exist.
// All operations are performed in-place.
func UpsertPod(coll *StatusCollection, pod *corev1.Pod) {
	if *coll == nil {
		*coll = make(StatusCollection, 0)
	}
	for i, p := range *coll {
		if p.Pod().UID == pod.UID {
			item := (*coll)[i]
			(*coll)[i] = StatusItem{pod: pod, err: item.err, status: item.status}
			return
		}
	}
	*coll = append(*coll, StatusItem{pod: pod, err: errors.New("missing status")})
}

// Pods returns all pods.
func (coll StatusCollection) Pods() []*corev1.Pod {
	return lo.Map(coll, func(status StatusItem, _ int) *corev1.Pod { return status.Pod() })
}

// SyncedPods returns the pods that are caught up with the chain tip.
func (coll StatusCollection) SyncedPods() []*corev1.Pod {
	var pods []*corev1.Pod
	for _, status := range coll {
		if status.err != nil {
			continue
		}
		if status.status.Result.SyncInfo.CatchingUp {
			continue
		}
		pods = append(pods, status.pod)
	}
	return pods
}
