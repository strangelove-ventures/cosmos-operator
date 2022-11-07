package snapshot

import (
	"fmt"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildJobs returns jobs to compress and upload data to an object storage.
func BuildJobs(crd *cosmosv1.HostedSnapshot) []*batchv1.Job {
	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		Spec: batchv1.JobSpec{
			// Set defaults
			ActiveDeadlineSeconds:   ptr(int64(12 * time.Hour.Seconds())),
			BackoffLimit:            ptr(int32(5)),
			TTLSecondsAfterFinished: ptr(int32(15 * time.Minute.Seconds())),

			Template: crd.Spec.PodTemplate,
		},
	}
	job.Labels = defaultLabels(crd)
	job.Namespace = crd.Namespace
	job.Name = jobName(crd)

	if v := crd.Spec.JobTemplate.ActiveDeadlineSeconds; v != nil {
		job.Spec.ActiveDeadlineSeconds = v
	}
	if v := crd.Spec.JobTemplate.BackoffLimit; v != nil {
		job.Spec.BackoffLimit = v
	}
	if v := crd.Spec.JobTemplate.TTLSecondsAfterFinished; v != nil {
		job.Spec.TTLSecondsAfterFinished = v
	}

	const volName = "snapshot"
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volName,
		VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvcName(crd),
		}},
	})

	if job.Spec.Template.Spec.RestartPolicy == "" {
		job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	}

	const chainHome = "/home/operator/cosmos"
	var (
		mount = corev1.VolumeMount{
			Name:      volName,
			MountPath: chainHome,
		}
		envVar = corev1.EnvVar{Name: "CHAIN_HOME", Value: chainHome}
	)

	for i := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].VolumeMounts = append(job.Spec.Template.Spec.Containers[i].VolumeMounts, mount)
		job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, envVar)
	}

	return []*batchv1.Job{&job}
}

// jobName returns the job's name created by the controller.
func jobName(crd *cosmosv1.HostedSnapshot) string {
	return kube.ToName(fmt.Sprintf("snapshot-%s", crd.Name))
}
