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
	"k8s.io/apimachinery/pkg/util/intstr"
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
	// Creates 1 pod per replica.
	PodTemplate CosmosPodSpec `json:"template"`

	// How to scale pods when performing an update.
	// +optional
	RolloutStrategy CosmosRolloutStrategy `json:"strategy"`

	// Will be used to create a stand-alone PVC to provision the volume.
	// One PVC per replica mapped and mounted to a corresponding pod.
	VolumeClaimTemplate CosmosPersistentVolumeClaim `json:"volumeClaimTemplate"`
}

type CosmosMetadata struct {
	// Labels are added to a resource. If there is a collision between labels the Operator creates, the Operator
	// labels take precedence.
	// +optional
	Labels map[string]string `json:"labels"`
	// Annotations are added to a resource. If there is a collision between annotations the Operator creates, the Operator
	// annotations take precedence.
	// +optional
	Annotations map[string]string `json:"annotations"`
}

type CosmosPodSpec struct {
	// Metadata is a subset of metav1.ObjectMeta applied to all pods.
	// +optional
	Metadata CosmosMetadata `json:"metadata"`

	// Image is the docker reference in "repository:tag" format. E.g. busybox:latest.
	// This is for the main container running the chain process.
	// +kubebuilder:validation:MinLength:=1
	Image string `json:"image"`

	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// This is for the main container running the chain process.
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// ImagePullSecrets is a list of references to secrets in the same namespace to use for pulling any images
	// in pods that reference this ServiceAccount. ImagePullSecrets are distinct from Secrets because Secrets
	// can be mounted in the pod, but ImagePullSecrets are only accessed by the kubelet.
	// More info: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
	// This is for the main container running the chain process.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// This is an advanced configuration option.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector"`

	// If specified, the pod's scheduling constraints
	// This is an advanced configuration option.
	// +optional
	Affinity *corev1.Affinity `json:"affinity"`

	// If specified, the pod's tolerations.
	// This is an advanced configuration option.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations"`

	// If specified, indicates the pod's priority. "system-node-critical" and
	// "system-cluster-critical" are two special keywords which indicate the
	// highest priorities with the former being the highest priority. Any other
	// name must be defined by creating a PriorityClass object with that name.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// This is an advanced configuration option.
	// +optional
	PriorityClassName string `json:"priorityClassName"`

	// The priority value. Various system components use this field to find the
	// priority of the pod. When Priority Admission Controller is enabled, it
	// prevents users from setting this field. The admission controller populates
	// this field from PriorityClassName.
	// The higher the value, the higher the priority.
	// This is an advanced configuration option.
	// +optional
	Priority *int32 `json:"priority"`

	// Resources describes the compute resource requirements.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources"`

	// Optional duration in seconds the pod needs to terminate gracefully. May be decreased in delete request.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// If this value is nil, the default grace period will be used instead.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// This is an advanced configuration option.
	// Defaults to 30 seconds.
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
}

// CosmosPersistentVolumeClaim describes the common attributes of storage devices
// and allows a Source for provider-specific attributes
type CosmosPersistentVolumeClaim struct {
	// storageClassName is the name of the StorageClass required by the claim.
	// For proper pod scheduling, it's highly recommended to set "volumeBindingMode: WaitForFirstConsumer" in the StorageClass.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
	// For GKE, recommended storage class is "premium-rwo".
	// This field is required.
	StorageClassName *string `json:"storageClassName,omitempty" protobuf:"bytes,5,opt,name=storageClassName"`

	// accessModes contains the desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	// This field is required.
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,1,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`

	// resources represents the minimum resources the volume should have.
	// This field is required.
	// If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements
	// that are lower than previous value but must still be higher than capacity recorded in the
	// status field of the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources
	// This field is required.
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`

	// volumeMode defines what type of volume is required by the claim.
	// Value of Filesystem is implied when not included in claim spec.
	// +optional
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode,omitempty" protobuf:"bytes,6,opt,name=volumeMode,casttype=PersistentVolumeMode"`

	// dataSource field can be used to specify either:
	// * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot)
	// * An existing PVC (PersistentVolumeClaim)
	// If the provisioner or an external controller can support the specified data source,
	// it will create a new volume based on the contents of the specified data source.
	// If the AnyVolumeDataSource feature gate is enabled, this field will always have
	// the same contents as the DataSourceRef field.
	// +optional
	DataSource *corev1.TypedLocalObjectReference `json:"dataSource,omitempty" protobuf:"bytes,7,opt,name=dataSource"`

	// dataSourceRef specifies the object from which to populate the volume with data, if a non-empty
	// volume is desired. This may be any local object from a non-empty API group (non
	// core object) or a PersistentVolumeClaim object.
	// When this field is specified, volume binding will only succeed if the type of
	// the specified object matches some installed volume populator or dynamic
	// provisioner.
	// This field will replace the functionality of the DataSource field and as such
	// if both fields are non-empty, they must have the same value. For backwards
	// compatibility, both fields (DataSource and DataSourceRef) will be set to the same
	// value automatically if one of them is empty and the other is non-empty.
	// There are two important differences between DataSource and DataSourceRef:
	// * While DataSource only allows two specific types of objects, DataSourceRef
	//   allows any non-core object, as well as PersistentVolumeClaim objects.
	// * While DataSource ignores disallowed values (dropping them), DataSourceRef
	//   preserves all values, and generates an error if a disallowed value is
	//   specified.
	// (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled.
	// +optional
	DataSourceRef *corev1.TypedLocalObjectReference `json:"dataSourceRef,omitempty" protobuf:"bytes,8,opt,name=dataSourceRef"`
}

// CosmosRolloutStrategy is an update strategy that can be shared between several Cosmos CRDs.
type CosmosRolloutStrategy struct {
	// The maximum number of pods that can be unavailable during an update.
	// Value can be an absolute number (ex: 5) or a percentage of desired pods (ex: 10%).
	// Absolute number is calculated from percentage by rounding down. The minimum max unavailable is 1.
	// Defaults to 25%.
	// Example: when this is set to 30%, pods are scaled down to 70% of desired pods
	// immediately when the rolling update starts. Once new pods are ready, pods
	// can be scaled down further, ensuring that the total number of pods available
	// at all times during the update is at least 70% of desired pods.
	// +kubebuilder:validation:XIntOrString
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty" protobuf:"bytes,1,opt,name=maxUnavailable"`
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
