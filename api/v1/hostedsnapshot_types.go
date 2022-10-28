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

	// The StorageClass to use when creating a temporary PVC for archiving and uploading the data archive to its
	// hosted location.
	StorageClassName string `json:"storageClassName"`
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
