package cosmos

import (
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

func (coll StatusCollection) Default() StatusCollection { return make(StatusCollection, 0) }

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
