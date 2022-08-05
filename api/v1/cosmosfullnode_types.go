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

// CosmosFullNodeSpec defines the desired state of CosmosFullNode
type CosmosFullNodeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Minimum:=1
	// Number of replicas to create.
	// Individual replicas have a consistent identity.
	Replicas int32 `json:"replicas"`

	// Template applied to all pods.
	PodTemplate CosmosFullNodePodTemplateSpec `json:"template"`
}

type CosmosFullNodePodTemplateSpec struct {
	// Image is the docker reference in "repository:tag" format. E.g. busybox:latest
	// +kubebuilder:validation:MinLength:=1
	Image string `json:"image"`

	// Resources describes the compute resource requirements.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources"`
}

// CosmosFullNodeStatus defines the observed state of CosmosFullNode
type CosmosFullNodeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CosmosFullNode is the Schema for the cosmosfullnodes API
type CosmosFullNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CosmosFullNodeSpec   `json:"spec,omitempty"`
	Status CosmosFullNodeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CosmosFullNodeList contains a list of CosmosFullNode
type CosmosFullNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CosmosFullNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CosmosFullNode{}, &CosmosFullNodeList{})
}
