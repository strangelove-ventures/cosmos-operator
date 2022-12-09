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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ScheduledVolumeSnapshot{}, &ScheduledVolumeSnapshotList{})
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ScheduledVolumeSnapshotSpec defines the desired state of ScheduledVolumeSnapshot
// Creates recurring VolumeSnapshots of a PVC managed by a CosmosFullNode.
// A VolumeSnapshot is a CRD (installed in GKE by default).
// See: https://kubernetes.io/docs/concepts/storage/volume-snapshots/
// This enables recurring, consistent backups.
// To prevent data corruption, a pod is temporarily deleted while the snapshot takes place which could take
// several minutes.
// Therefore, if you create a ScheduledVolumeSnapshot, you must use replica count >= 2 to prevent downtime.
// If <= 1 pod in a ready state, the controller will not temporarily delete the pod. The controller makes every
// effort to prevent downtime.
// Only 1 VolumeSnapshot is created at a time, so at most only 1 pod is temporarily deleted.
// Multiple, parallel VolumeSnapshots are not supported.
type ScheduledVolumeSnapshotSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Reference to the source CosmosFullNode.
	SourceRef ObjectRef `json:"sourceRef"`

	// A crontab schedule using the standard as described in https://en.wikipedia.org/wiki/Cron.
	// See https://crontab.guru for format.
	// Kubernetes providers rate limit VolumeSnapshot creation. Therefore, setting a crontab that's
	// too frequent may result in rate limiting errors.
	Schedule string `json:"schedule"`

	// The name of the VolumeSnapshotClass to use when creating snapshots.
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName"`

	// Minimum number of CosmosFullNode pods that must be ready before creating a VolumeSnapshot.
	// This controller gracefully deletes a pod while taking a snapshot. Then recreates the pod once the
	// snapshot is complete.
	// This way, the snapshot has the highest possible data integrity.
	// Defaults to 2.
	// Warning: If set to 1, you will experience downtime.
	// +optional
	// +kubebuilder:validation:Minimum:=1
	MinAvailable int32 `json:"minAvailable"`

	// The number of recent VolumeSnapshots to keep.
	// Default is 3.
	// +optional
	// +kubebuilder:validation:Minimum:=0
	Limit int32 `json:"limit"`
}

type ObjectRef struct {
	// Name of the object, metadata.name
	Name string `json:"name"`
	// Namespace of the object, metadata.namespace
	Namespace string `json:"namespace"`
}

// ScheduledVolumeSnapshotStatus defines the observed state of ScheduledVolumeSnapshot
type ScheduledVolumeSnapshotStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration"`

	// A generic message for the user. May contain errors.
	// +optional
	StatusMessage *string `json:"status"`

	// The date when the CRD was created.
	// Used as a reference when calculating the next time to create a snapshot.
	CreatedAt metav1.Time `json:"createdAt"`

	// The most recent volume snapshot created by the controller.
	// +optional
	LastSnapshot *VolumeSnapshotStatus `json:"lastSnapshot"`
}

type VolumeSnapshotStatus struct {
	// The name of the created VolumeSnapshot.
	Name string `json:"name"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ScheduledVolumeSnapshot is the Schema for the scheduledvolumesnapshots API
type ScheduledVolumeSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScheduledVolumeSnapshotSpec   `json:"spec,omitempty"`
	Status ScheduledVolumeSnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ScheduledVolumeSnapshotList contains a list of ScheduledVolumeSnapshot
type ScheduledVolumeSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScheduledVolumeSnapshot `json:"items"`
}
