/*
Copyright 2022 Strangelove Ventures LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&StatefulJob{}, &StatefulJobList{})
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StatefulJobSpec defines the desired state of StatefulJob
type StatefulJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The selector to target VolumeSnapshots.
	Selector map[string]string `json:"selector"`

	// Interval at which the controller runs snapshot job with pvc.
	// Expressed as a duration string, e.g. 1.5h, 24h, 12h.
	// Defaults to 24h.
	// +optional
	Interval metav1.Duration `json:"interval"`

	// Specification of the desired behavior of the job.
	// +optional
	JobTemplate JobTemplateSpec `json:"jobTemplate"`

	// Specification of the desired behavior of the job's pod.
	// You should include container commands and args to perform the upload of data to a remote location like an
	// object store such as S3 or GCS.
	// Volumes will be injected and mounted into every container in the spec.
	// Working directory will be /home/operator.
	// The chain directory will be /home/operator/cosmos and set as env var $CHAIN_HOME.
	// If not set, pod's restart policy defaults to Never.
	PodTemplate corev1.PodTemplateSpec `json:"podTemplate"`

	// Specification for the PVC associated with the job.
	VolumeClaimTemplate StatefulJobVolumeClaimTemplate `json:"volumeClaimTemplate"`
}

// JobTemplateSpec is a subset of batchv1.JobSpec.
type JobTemplateSpec struct {
	// Specifies the duration in seconds relative to the startTime that the job
	// may be continuously active before the system tries to terminate it; value
	// must be positive integer.
	// Do not set too short or you will run into PVC/VolumeSnapshot provisioning rate limits.
	// Defaults to 24 hours.
	// +kubebuilder:validation:Minimum:=1
	// +optional
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds"`

	// Specifies the number of retries before marking this job failed.
	// Defaults to 5.
	// +kubebuilder:validation:Minimum:=0
	// +optional
	BackoffLimit *int32 `json:"backoffLimit"`

	// Limits the lifetime of a Job that has finished
	// execution (either Complete or Failed). If this field is set,
	// ttlSecondsAfterFinished after the Job finishes, it is eligible to be
	// automatically deleted. When the Job is being deleted, its lifecycle
	// guarantees (e.g. finalizers) will be honored. If this field is set to zero,
	// the Job becomes eligible to be deleted immediately after it finishes.
	// Defaults to 15 minutes to allow some time to inspect logs.
	// +kubebuilder:validation:Minimum:=0
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished"`
}

// StatefulJobVolumeClaimTemplate is a subset of a PersistentVolumeClaimTemplate
type StatefulJobVolumeClaimTemplate struct {
	// The StorageClass to use when creating a temporary PVC for processing the data.
	// On GKE, the StorageClass must be the same as the PVC's StorageClass from which the
	// VolumeSnapshot was created.
	StorageClassName string `json:"storageClassName"`

	// The desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	// Defaults to ReadWriteOnce.
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes"`
}

// StatefulJobStatus defines the observed state of StatefulJob
type StatefulJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration"`

	// A generic message for the user. May contain errors.
	// +optional
	StatusMessage *string `json:"status"`

	// Last 5 job statuses created by the controller ordered by more recent jobs.
	// +optional
	JobHistory []batchv1.JobStatus `json:"jobHistory"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// StatefulJob is the Schema for the statefuljobs API
type StatefulJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StatefulJobSpec   `json:"spec,omitempty"`
	Status StatefulJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StatefulJobList contains a list of StatefulJob
type StatefulJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StatefulJob `json:"items"`
}
