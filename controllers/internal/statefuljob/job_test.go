package statefuljob

import (
	"testing"

	"github.com/samber/lo"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildJobs(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		crd := cosmosalpha.StatefulJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "axelar",
				Namespace: "test",
			},
			Spec: cosmosalpha.StatefulJobSpec{
				JobTemplate: cosmosalpha.JobTemplateSpec{
					ActiveDeadlineSeconds:   ptr(int64(20)),
					BackoffLimit:            ptr(int32(1)),
					TTLSecondsAfterFinished: ptr(int32(10)),
				},
				PodTemplate: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyAlways,
					},
				},
			},
		}

		jobs := BuildJobs(&crd)
		require.Len(t, jobs, 1)
		got := jobs[0]

		require.Equal(t, "test", got.Namespace)
		require.Equal(t, "axelar", got.Name)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/component":  "StatefulJob",
		}
		require.Equal(t, wantLabels, got.Labels)

		require.EqualValues(t, 20, *got.Spec.ActiveDeadlineSeconds)
		require.EqualValues(t, 1, *got.Spec.BackoffLimit)
		require.EqualValues(t, 10, *got.Spec.TTLSecondsAfterFinished)

		require.Nil(t, got.Spec.Parallelism)
		require.Equal(t, corev1.RestartPolicyAlways, got.Spec.Template.Spec.RestartPolicy)
	})

	t.Run("defaults", func(t *testing.T) {
		var crd cosmosalpha.StatefulJob

		jobs := BuildJobs(&crd)
		require.Len(t, jobs, 1)

		got := jobs[0]

		require.EqualValues(t, 900, *got.Spec.TTLSecondsAfterFinished)
		require.EqualValues(t, 5, *got.Spec.BackoffLimit)
		require.EqualValues(t, 86_400, *got.Spec.ActiveDeadlineSeconds)

		require.Equal(t, corev1.RestartPolicyNever, got.Spec.Template.Spec.RestartPolicy)
		require.Len(t, got.Spec.Template.Spec.Volumes, 1)
	})

	t.Run("volumes", func(t *testing.T) {
		crd := cosmosalpha.StatefulJob{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cosmoshub",
			},
			Spec: cosmosalpha.StatefulJobSpec{
				PodTemplate: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Volumes: make([]corev1.Volume, 2),
					},
				},
			},
		}

		jobs := BuildJobs(&crd)
		require.Len(t, jobs, 1)
		got := jobs[0]

		require.Len(t, got.Spec.Template.Spec.Volumes, 3)
		gotVol, err := lo.Last(got.Spec.Template.Spec.Volumes)
		require.NoError(t, err)
		require.Equal(t, "snapshot", gotVol.Name)
		require.Equal(t, "cosmoshub", gotVol.VolumeSource.PersistentVolumeClaim.ClaimName)
	})
}
