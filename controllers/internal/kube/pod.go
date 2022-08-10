package kube

import (
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
)

// IsPodAvailable returns true if a pod is available; false otherwise.
// Precondition for an available pod is that it must be ready.
// Additionally, there are two cases when a pod can be considered available:
// 1. minReady == 0, or
// 2. LastTransitionTime (is set) + minReadySeconds < current time
//
// Much of this code was vendored from the kubernetes codebase.
func IsPodAvailable(pod *corev1.Pod, minReady time.Duration, now time.Time) bool {
	if !isPodReadyConditionTrue(pod.Status) {
		return false
	}

	c := getPodReadyCondition(pod.Status)
	if minReady == 0 || (!c.LastTransitionTime.IsZero() && c.LastTransitionTime.Add(minReady).Before(now)) {
		return true
	}
	return false
}

// isPodReadyConditionTrue returns true if a pod is ready; false otherwise.
func isPodReadyConditionTrue(status corev1.PodStatus) bool {
	condition := getPodReadyCondition(status)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// getPodReadyCondition extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func getPodReadyCondition(status corev1.PodStatus) *corev1.PodCondition {
	_, condition := getPodCondition(&status, corev1.PodReady)
	return condition
}

// getPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func getPodCondition(status *corev1.PodStatus, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return getPodConditionFromList(status.Conditions, conditionType)
}

// getPodConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func getPodConditionFromList(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

// AvailablePods returns pods which are available as defined in IsPodAvailable.
func AvailablePods(pods []*corev1.Pod, minReady time.Duration, now time.Time) []*corev1.Pod {
	return lo.Filter(pods, func(p *corev1.Pod, _ int) bool { return IsPodAvailable(p, minReady, now) })
}
