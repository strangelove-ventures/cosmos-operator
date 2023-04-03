package kube

import (
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestFindOrDefault(t *testing.T) {
	pods := lo.Map(lo.Range(3), func(i int, _ int) *corev1.Pod {
		var pod corev1.Pod
		pod.Name = fmt.Sprintf("pod-%d", i)
		pod.Namespace = "default"
		pod.Spec.Volumes = []corev1.Volume{
			{Name: fmt.Sprintf("vol-%d", i)},
		}
		return &pod
	})

	t.Run("found", func(t *testing.T) {
		var cmp corev1.Pod
		cmp.Name = "pod-1"
		cmp.Namespace = "default"

		got := FindOrDefault(pods, &cmp)

		var want corev1.Pod
		want.Name = "pod-1"
		want.Namespace = "default"
		want.Annotations = map[string]string{}
		want.Labels = map[string]string{}
		want.Spec.Volumes = []corev1.Volume{
			{Name: fmt.Sprintf("vol-1")},
		}

		require.Equal(t, &want, got)
		require.NotSame(t, pods[1], got)

		require.NotNil(t, got.Labels)
		require.NotNil(t, got.Annotations)
	})

	t.Run("not found", func(t *testing.T) {
		var cmp corev1.Pod
		cmp.Name = "pod-1"
		cmp.Namespace = "notsame"

		got := FindOrDefault(pods, &cmp)

		want := cmp.DeepCopy()
		want.Annotations = map[string]string{}
		want.Labels = map[string]string{}

		require.Equal(t, want, got)
		require.NotSame(t, &cmp, got)
	})
}
