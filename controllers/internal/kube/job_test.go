package kube

import (
	"testing"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestIsJobFinished(t *testing.T) {
	for _, tt := range []struct {
		Job  batchv1.JobCondition
		Want bool
	}{
		{batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}, true},
		{batchv1.JobCondition{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}, true},

		{batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionUnknown}, false},
		{batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionFalse}, false},
		{batchv1.JobCondition{Type: batchv1.JobSuspended, Status: corev1.ConditionTrue}, false},
	} {
		var job batchv1.Job
		job.Status.Conditions = []batchv1.JobCondition{{Type: "test", Status: "ignored"}, tt.Job}

		require.Equal(t, tt.Want, IsJobFinished(&job), tt)
	}
}
