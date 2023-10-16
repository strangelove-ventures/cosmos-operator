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
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ScheduledVolumeSnapshot{}, &ScheduledVolumeSnapshotList{})
}

// ScheduledVolumeSnapshotController is the canonical controller name.
const ScheduledVolumeSnapshotController = "ScheduledVolumeSnapshot"

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
	// This field is immutable. If you change the fullnode, you may encounter undefined behavior.
	// The CosmosFullNode must be in the same namespace as the ScheduledVolumeSnapshot.
	// Instead delete the ScheduledVolumeSnapshot and create a new one with the correct fullNodeRef.
	FullNodeRef LocalFullNodeRef `json:"fullNodeRef"`

	// A crontab schedule using the standard as described in https://en.wikipedia.org/wiki/Cron.
	// See https://crontab.guru for format.
	// Kubernetes providers rate limit VolumeSnapshot creation. Therefore, setting a crontab that's
	// too frequent may result in rate limiting errors.
	Schedule string `json:"schedule"`

	// The name of the VolumeSnapshotClass to use when creating snapshots.
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName"`

	// If true, the controller will temporarily delete the candidate pod before taking a snapshot of the pod's associated PVC.
	// This option prevents writes to the PVC, ensuring the highest possible data integrity.
	// Once the snapshot is created, the pod will be restored.
	// +optional
	DeletePod bool `json:"deletePod"`

	// Minimum number of CosmosFullNode pods that must be ready before creating a VolumeSnapshot.
	// In the future, this field will have no effect unless spec.deletePod=true.
	// This controller gracefully deletes a pod while taking a snapshot. Then recreates the pod once the
	// snapshot is complete.
	// This way, the snapshot has the highest possible data integrity.
	// Defaults to 2.
	// Warning: If set to 1, you will experience downtime.
	// +optional
	// +kubebuilder:validation:Minimum:=1
	MinAvailable int32 `json:"minAvailable"`

	// The number of recent VolumeSnapshots to keep.
	// Defaults to 3.
	// +optional
	// +kubebuilder:validation:Minimum:=1
	Limit int32 `json:"limit"`

	// If true, the controller will not create any VolumeSnapshots.
	// This allows you to disable creation of VolumeSnapshots without deleting the ScheduledVolumeSnapshot resource.
	// This pattern works better when using tools such as Kustomzie.
	// If a pod is temporarily deleted, it will be restored.
	// +optional
	Suspend bool `json:"suspend"`
}

type LocalFullNodeRef struct {
	// Name of the object, metadata.name
	Name string `json:"name"`
	// DEPRECATED: CosmosFullNode must be in the same namespace as the ScheduledVolumeSnapshot. This field is ignored.
	// +optional
	Namespace string `json:"namespace"`

	// Index of the pod to snapshot. If not provided, will do any pod in the CosmosFullNode.
	// Useful when snapshots are local to the same node as the pod, requiring snapshots across multiple pods/nodes.
	// +optional
	Ordinal *int32 `json:"ordinal"`
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

	// The phase of the controller.
	Phase SnapshotPhase `json:"phase"`

	// The date when the CRD was created.
	// Used as a reference when calculating the next time to create a snapshot.
	CreatedAt metav1.Time `json:"createdAt"`

	// The pod/pvc pair of the CosmosFullNode from which to make a VolumeSnapshot.
	// +optional
	Candidate *SnapshotCandidate `json:"candidate"`

	// The most recent volume snapshot created by the controller.
	// +optional
	LastSnapshot *VolumeSnapshotStatus `json:"lastSnapshot"`
}

type SnapshotCandidate struct {
	PodName string `json:"podName"`
	PVCName string `json:"pvcName"`

	// +optional
	PodLabels map[string]string `json:"podLabels"`
}

type SnapshotPhase string

// These values are persisted. Do not change arbitrarily.
const (
	// SnapshotPhaseWaitingForNext means waiting for the next scheduled time to start the snapshot creation process.
	SnapshotPhaseWaitingForNext SnapshotPhase = "WaitingForNext"

	// SnapshotPhaseFindingCandidate is finding a pod/pvc candidate from which to create a VolumeSnapshot.
	SnapshotPhaseFindingCandidate SnapshotPhase = "FindingCandidate"

	// SnapshotPhaseDeletingPod signals the fullNodeRef to delete the candidate pod. This allows taking a VolumeSnapshot
	// on a "quiet" PVC, with no processes writing to it.
	SnapshotPhaseDeletingPod SnapshotPhase = "DeletingPod"

	// SnapshotPhaseWaitingForPodDeletion indicates controller is waiting for the fullNodeRef to delete the candidate pod.
	SnapshotPhaseWaitingForPodDeletion SnapshotPhase = "WaitingForPodDeletion"

	// SnapshotPhaseCreating indicates controller found a candidate and will now create a VolumeSnapshot from the PVC.
	SnapshotPhaseCreating SnapshotPhase = "CreatingSnapshot"

	// SnapshotPhaseWaitingForCreation means the VolumeSnapshot has been created and the controller is waiting for
	// the VolumeSnapshot to become ready for use.
	SnapshotPhaseWaitingForCreation SnapshotPhase = "WaitingForSnapshotCreation"

	// SnapshotPhaseRestorePod signals the fullNodeRef it can recreate the temporarily deleted pod.
	SnapshotPhaseRestorePod SnapshotPhase = "RestoringPod"

	// SnapshotPhaseSuspended means the controller is not creating snapshots. Suspended by the user.
	SnapshotPhaseSuspended SnapshotPhase = "Suspended"

	// SnapshotPhaseMissingCRDs means the controller is not creating snapshots. The required VolumeSnapshot CRDs are missing.
	SnapshotPhaseMissingCRDs SnapshotPhase = "MissingCRDs"
)

type VolumeSnapshotStatus struct {
	// The name of the created VolumeSnapshot.
	Name string `json:"name"`

	// The time the controller created the VolumeSnapshot.
	StartedAt metav1.Time `json:"startedAt"`

	// The last VolumeSnapshot's status
	// +optional
	Status *snapshotv1.VolumeSnapshotStatus `json:"status"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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
