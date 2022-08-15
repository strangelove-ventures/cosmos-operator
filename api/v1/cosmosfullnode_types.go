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

	// Blockchain-specific configuration for the fullnode.
	ChainConfig CosmosChainConfig `json:"chain"`

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

// CosmosFullNodeStatus defines the observed state of CosmosFullNode
type CosmosFullNodeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds"`
}

// CosmosPersistentVolumeClaim describes the common attributes of storage devices
// and allows a Source for provider-specific attributes
type CosmosPersistentVolumeClaim struct {
	// storageClassName is the name of the StorageClass required by the claim.
	// For proper pod scheduling, it's highly recommended to set "volumeBindingMode: WaitForFirstConsumer" in the StorageClass.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
	// For GKE, recommended storage class is "premium-rwo".
	// This field is immutable. Updating this field requires manually deleting the PVC.
	// This field is required.
	StorageClassName string `json:"storageClassName"`

	// resources represents the minimum resources the volume should have.
	// If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements
	// that are lower than previous value but must still be higher than capacity recorded in the
	// status field of the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources
	// Updating the storage size is allowed but the StorageClass must support file system resizing.
	// Only increasing storage is permitted.
	// This field is required.
	Resources corev1.ResourceRequirements `json:"resources"`

	// accessModes contain the desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	// If not specified, defaults to ReadWriteOnce.
	// This field is immutable. Updating this field requires manually deleting the PVC.
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes"`

	// volumeMode defines what type of volume is required by the claim.
	// Value of Filesystem is implied when not included in claim spec.
	// This field is immutable. Updating this field requires manually deleting the PVC.
	// +optional
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode"`
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
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable"`
}

type CosmosChainConfig struct {
	// Genesis file chain-id.
	// +kubebuilder:validation:MinLength:=1
	ChainID string `json:"chainID"`

	// Binary name which runs commands. E.g. gaiad, junod, osmosisd
	// +kubebuilder:validation:MinLength:=1
	Binary string `json:"binary"`

	// Tendermint configuration applied to config.toml.
	Tendermint CosmosTendermintConfig `json:"config"`

	// App configuration applied to app.toml.
	App CosmosAppConfig `json:"appConfig"`
}

// CosmosTendermintConfig configures the tendermint config.toml.
type CosmosTendermintConfig struct {
	// p2p address to advertise for peers to dial.
	// Example: 159.89.10.97 or my.domain.com.
	// Omit the port. Operator will configure the port appropriately (26656).
	// +kubebuilder:validation:MinLength:=1
	ExternalAddress string `json:"externalAddress"`

	// List of p2p nodes in <ID>@<IP>:<PORT> format to keep persistent p2p connections.
	// See https://docs.tendermint.com/master/spec/p2p/peer.html and
	// https://docs.tendermint.com/master/spec/p3p/config.html#persistent-peers.
	// +kubebuilder:validation:MinItems:=1
	PersistentPeers []string `json:"peers"`

	// List of p2p seed nodes in <ID>@<IP>:<PORT> format.
	// See https://docs.tendermint.com/master/spec/p2p/config.html#seeds and
	// https://docs.tendermint.com/master/spec/p2p/node.html#seeds.
	// +kubebuilder:validation:MinItems:=1
	Seeds []string `json:"seeds"`

	// p2p maximum number of inbound peers.
	// +kubebuilder:validation:Minimum:=1
	MaxInboundPeers int32 `json:"maxInboundPeers"`

	// p2p maximum number of outbound peers.
	// +kubebuilder:validation:Minimum:=1
	MaxOutboundPeers int32 `json:"maxOutboundPeers"`

	// rpc list of origins a cross-domain request can be executed from.
	// Default value '[]' disables cors support.
	// Use '["*"]' to allow any origin.
	// +optional
	CorsAllowedOrigins []string `json:"corsAllowedOrigins"`

	// Custom tendermint config toml.
	// Values entered here take precedence over all other configuration.
	// Must be valid toml.
	// +optional
	TomlOverrides *string `json:"overrides"`

	// One of error, info, debug, trace
	// If not set, defaults to info.
	// +optional
	LogLevel *string `json:"logLevel"`

	// One of plain or json.
	// If not set, defaults to plain.
	// +optional
	LogFormat *string `json:"logFormat"`
}

type CosmosAppConfig struct {
	// The minimum gas prices a validator is willing to accept for processing a
	// transaction. A transaction's fees must meet the minimum of any denomination
	// specified in this config (e.g. 0.25token1;0.0001token2).
	// +kubebuilder:validation:MinLength:=1
	MinGasPrice string `json:"minGasPrice"`
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
