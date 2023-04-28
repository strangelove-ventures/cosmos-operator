package cosmos

import corev1 "k8s.io/api/core/v1"

// PodStatus is a pod paired with its tendermint/cometbft status.
type PodStatus struct {
	pod    *corev1.Pod
	status TendermintStatus
	err    error
}

// Pod returns the pod.
func (status PodStatus) Pod() *corev1.Pod {
	return status.pod
}

// Status returns the tendermint/cometbft status or an error if the status could not be fetched.
func (status PodStatus) Status() (TendermintStatus, error) {
	return status.status, status.err
}

// StatusCollection is a list of pods and tendermint status associated with the pod.
type StatusCollection []PodStatus

// SyncedPods returns the pods that are not catching up.
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
