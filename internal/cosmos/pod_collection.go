package cosmos

import (
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
)

// Pod is a pod paired with its tendermint/cometbft status.
type Pod struct {
	pod    *corev1.Pod
	status TendermintStatus
	err    error
}

// Pod returns the pod.
func (status Pod) Pod() *corev1.Pod {
	return status.pod
}

// Status returns the tendermint/cometbft status or an error if the status could not be fetched.
func (status Pod) Status() (TendermintStatus, error) {
	return status.status, status.err
}

// PodCollection is a list of pods and tendermint status associated with the pod.
type PodCollection []Pod

func (coll PodCollection) Default() PodCollection { return make(PodCollection, 0) }

// Pods returns all pods.
func (coll PodCollection) Pods() []*corev1.Pod {
	return lo.Map(coll, func(status Pod, _ int) *corev1.Pod { return status.Pod() })
}

// SyncedPods returns the pods that are not catching up.
func (coll PodCollection) SyncedPods() []*corev1.Pod {
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
