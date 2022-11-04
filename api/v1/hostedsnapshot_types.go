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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HostedSnapshotSpec defines the desired state of HostedSnapshot
type HostedSnapshotSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The selector to target VolumeSnapshots.
	Selector map[string]string `json:"selector"`

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
	VolumeClaimTemplate SnapshotVolumeClaimTemplate `json:"volumeClaimTemplate"`
}

// JobTemplateSpec is a subset of batchv1.JobSpec.
type JobTemplateSpec struct {
	// Specifies the duration in seconds relative to the startTime that the job
	// may be continuously active before the system tries to terminate it; value
	// must be positive integer.
	// Defaults to 12 hours.
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

// SnapshotVolumeClaimTemplate is a subset of a PersistentVolumeClaimTemplate
type SnapshotVolumeClaimTemplate struct {
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

// HostedSnapshotStatus defines the observed state of HostedSnapshot
type HostedSnapshotStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//TODO(nix): Possible fields observedGeneration, volumeSnapshotName (which one we used), jobName (ref to job)
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HostedSnapshot is the Schema for the hostedsnapshots API
type HostedSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostedSnapshotSpec   `json:"spec,omitempty"`
	Status HostedSnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HostedSnapshotList contains a list of HostedSnapshot
type HostedSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostedSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostedSnapshot{}, &HostedSnapshotList{})
}
