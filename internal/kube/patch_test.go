package kube

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStrategicPatch(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		target := &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Labels: map[string]string{
					"app": "myapp",
					"foo": "bar",
				},
			},
			Spec: corev1.PodSpec{
				NodeSelector:  map[string]string{"test": "value"},
				RestartPolicy: corev1.RestartPolicyAlways,
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:v1",
					},
					{
						Name:  "second",
						Image: "v2",
					},
				},
			},
		}

		patch := &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "CHANGED",
				},
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:CHANGED",
					},
				},
			},
		}

		err := ApplyStrategicPatch(target, patch)

		require.NoError(t, err)

		want := &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Labels: map[string]string{
					"app": "CHANGED",
					"foo": "bar",
				},
			},
			Spec: corev1.PodSpec{
				NodeSelector:  map[string]string{"test": "value"},
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:CHANGED",
					},
					{
						Name:  "second",
						Image: "v2",
					},
				},
			},
		}
		require.Equal(t, want, target)
	})

	t.Run("identity", func(t *testing.T) {
		obj := &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			Spec: corev1.PodSpec{
				NodeSelector:  map[string]string{"test": "value"},
				RestartPolicy: corev1.RestartPolicyAlways,
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:v1",
					},
					{
						Name:  "second",
						Image: "v2",
					},
				},
			},
		}

		want := obj.DeepCopy()

		err := ApplyStrategicPatch(obj, obj)
		require.NoError(t, err)
		require.Equal(t, want, obj)
	})
}
