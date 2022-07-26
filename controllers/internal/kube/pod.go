package kube

import (
	corev1 "k8s.io/api/core/v1"
)

// PodIsReady returns true if the pod status is in Ready state, meaning it can be added to a Service.
func PodIsReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
