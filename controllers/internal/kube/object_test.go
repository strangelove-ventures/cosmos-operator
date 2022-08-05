package kube

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FuzzObjectHasChanges(f *testing.F) {
	newPod := func(name, ns, labelVal, annotationVal string) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   ns,
				Labels:      map[string]string{"group/test": labelVal},
				Annotations: map[string]string{"group/ordinal": annotationVal},
			},
		}
	}
	f.Add("pod-0", "default", "value", "0")

	f.Fuzz(func(t *testing.T, name, ns, labelVal, annotationVal string) {
		var (
			pod1 = newPod(name, ns, labelVal, annotationVal)
			pod2 = newPod(name, ns, labelVal, annotationVal)
		)

		seed := []string{name, ns, labelVal, annotationVal}

		require.False(t, ObjectHasChanges(&pod1, &pod2), seed)
		require.False(t, ObjectHasChanges(&pod2, &pod1), seed)

		pod1.Name = name + "_changed"
		require.True(t, ObjectHasChanges(&pod1, &pod2), seed)
		require.True(t, ObjectHasChanges(&pod2, &pod1), seed)

		pod1.Name = name
		require.False(t, ObjectHasChanges(&pod1, &pod2), seed)

		pod1.Namespace = ns + "_changed"
		require.True(t, ObjectHasChanges(&pod1, &pod2), seed)
		require.True(t, ObjectHasChanges(&pod2, &pod1), seed)

		pod1.Namespace = ns
		require.False(t, ObjectHasChanges(&pod1, &pod2), seed)

		pod1.Labels["changed"] = "true"
		require.True(t, ObjectHasChanges(&pod1, &pod2), seed)
		require.True(t, ObjectHasChanges(&pod2, &pod1), seed)

		pod1.Labels = pod2.Labels
		require.False(t, ObjectHasChanges(&pod1, &pod2), seed)

		pod1.Annotations["changed"] = "true"
		require.True(t, ObjectHasChanges(&pod1, &pod2), seed)
		require.True(t, ObjectHasChanges(&pod2, &pod1), seed)

		pod1.Annotations = pod2.Annotations
		require.False(t, ObjectHasChanges(&pod1, &pod2), seed)
	})
}
