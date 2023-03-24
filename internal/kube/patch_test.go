package kube

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStrategicPatch(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		origObj := &corev1.PodTemplateSpec{
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

		patch := origObj.DeepCopy()
		patch.Labels["app"] = "CHANGED"
		patch.Spec.RestartPolicy = corev1.RestartPolicyNever
		patch.Spec.Containers = []corev1.Container{
			{
				Name:  "app",
				Image: "myapp:CHANGED",
			},
		}

		//patch := &corev1.PodTemplateSpec{
		//	ObjectMeta: metav1.ObjectMeta{
		//		Labels: map[string]string{
		//			"app": "CHANGED",
		//		},
		//	},
		//	Spec: corev1.PodSpec{
		//		RestartPolicy: corev1.RestartPolicyNever,
		//		Containers: []corev1.Container{
		//			{
		//				Name:  "app",
		//				Image: "myapp:CHANGED",
		//			},
		//		},
		//	},
		//}

		err := ApplyStrategicPatch(origObj, patch)

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
		require.Equal(t, want, origObj)
	})

	t.Run("identity", func(t *testing.T) {

	})
}
