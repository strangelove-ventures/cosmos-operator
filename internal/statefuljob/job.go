package statefuljob

import (
	"time"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildJobs returns jobs to compress and upload data to an object storage.
func BuildJobs(crd *cosmosalpha.StatefulJob) []*batchv1.Job {
	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		Spec: batchv1.JobSpec{
			// Set defaults
			ActiveDeadlineSeconds:   ptr(int64(24 * time.Hour.Seconds())),
			BackoffLimit:            ptr(int32(5)),
			TTLSecondsAfterFinished: ptr(int32(15 * time.Minute.Seconds())),

			Template: crd.Spec.PodTemplate,
		},
	}
	job.Labels = defaultLabels()
	job.Namespace = crd.Namespace
	job.Name = ResourceName(crd)

	if v := crd.Spec.JobTemplate.ActiveDeadlineSeconds; v != nil {
		job.Spec.ActiveDeadlineSeconds = v
	}
	if v := crd.Spec.JobTemplate.BackoffLimit; v != nil {
		job.Spec.BackoffLimit = v
	}
	if v := crd.Spec.JobTemplate.TTLSecondsAfterFinished; v != nil {
		job.Spec.TTLSecondsAfterFinished = v
	}

	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "snapshot",
		VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: ResourceName(crd),
		}},
	})

	if job.Spec.Template.Spec.RestartPolicy == "" {
		job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	}

	return []*batchv1.Job{&job}
}
