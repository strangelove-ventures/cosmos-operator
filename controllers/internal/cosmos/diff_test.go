package cosmos

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	current := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "hub-0"},
			Spec:       corev1.PodSpec{},
			Status:     corev1.PodStatus{},
		},
	}

	// Purposefully unordered
	want := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "hub-1"},
			Spec:       corev1.PodSpec{},
			Status:     corev1.PodStatus{},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "hub-0"},
			Spec:       corev1.PodSpec{},
			Status:     corev1.PodStatus{},
		},
	}

	diff := NewDiff(current, want)

	require.Empty(t, diff.Deletes())
	require.Empty(t, diff.Updates())

	require.Len(t, diff.Creates(), 1)
	require.Equal(t, diff.Creates()[0].Name, "hub-1")
}
