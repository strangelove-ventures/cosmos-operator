package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestAdapt(t *testing.T) {
	t.Parallel()

	var pod corev1.Pod
	pod.Name = "test"
	pod.Namespace = "default"
	pod.Spec.NodeName = "node-1"

	got := Adapt(&pod, 1)

	require.Same(t, &pod, got.Object())
	require.Equal(t, int64(1), got.Ordinal())
	require.NotEmpty(t, got.Revision())

	got2 := Adapt(&pod, 1)
	require.Equal(t, got.Revision(), got2.Revision())

	pod.Labels = map[string]string{"foo": "bar"}
	got3 := Adapt(&pod, 1)
	require.NotEqual(t, got.Revision(), got3.Revision())
}
