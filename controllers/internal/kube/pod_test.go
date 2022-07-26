package kube

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestPodIsReady(t *testing.T) {
	for _, tt := range []struct {
		Conditions []corev1.PodCondition
		Want       bool
	}{
		// Empty states
		{nil, false},
		{make([]corev1.PodCondition, 0), false},

		// Not ready
		{[]corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionUnknown},
		}, false},
		{[]corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionFalse},
		}, false},
		{[]corev1.PodCondition{
			{Type: corev1.PodInitialized, Status: corev1.ConditionFalse},
		}, false},

		// Is ready
		{[]corev1.PodCondition{
			{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
		}, true},
		{[]corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
		}, true},
	} {
		pod := corev1.Pod{
			Status: corev1.PodStatus{Conditions: tt.Conditions},
		}

		require.Equal(t, tt.Want, PodIsReady(&pod), tt)
	}
}
