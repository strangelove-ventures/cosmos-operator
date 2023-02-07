package kube

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPodAvailable(t *testing.T) {
	now := time.Now()

	for _, tt := range []struct {
		Condition corev1.PodCondition
		MinReady  time.Duration
		Want      bool
	}{
		// Not available
		{corev1.PodCondition{}, 0, false},
		{corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionFalse}, 0, false},
		{corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionUnknown}, 0, false},
		{corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(now)}, time.Second, false},

		// Available
		{corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue}, 0, true},
		{corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(now.Add(-5 * time.Second))}, 2 * time.Second, true},
	} {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
					{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
					{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
				},
			},
		}
		pod.Status.Conditions = append(pod.Status.Conditions, tt.Condition)

		got := IsPodAvailable(pod, tt.MinReady, now)
		require.Equal(t, tt.Want, got, tt)
	}
}
