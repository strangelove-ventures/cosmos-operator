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

// FullNodeSpec defines the desired state of CosmosFullNode
type FullNodeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Number of replicas to create.
	// Individual replicas have a consistent identity.
	// +kubebuilder:validation:Minimum:=0
	Replicas int32 `json:"replicas"`

	// Blockchain-specific configuration.
	ChainConfig ChainConfig `json:"chain"`

	// Template applied to all pods.
	// Creates 1 pod per replica.
	PodTemplate PodSpec `json:"podTemplate"`

	// How to scale pods when performing an update.
	// +optional
	RolloutStrategy RolloutStrategy `json:"strategy"`

	// Will be used to create a stand-alone PVC to provision the volume.
	// One PVC per replica mapped and mounted to a corresponding pod.
	VolumeClaimTemplate PersistentVolumeClaimSpec `json:"volumeClaimTemplate"`

	// Configure Operator created services. A singe rpc service is created for load balancing api, grpc, rpc, etc. requests.
	// This allows a k8s admin to use the service in an Ingress, for example.
	// Additionally, multiple p2p services are created for tendermint peer exchange.
	// +optional
	Service ServiceSpec `json:"service"`
}

// FullNodeStatus defines the observed state of CosmosFullNode
type FullNodeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

type FullNodeMetadata struct {
	// Labels are added to a resource. If there is a collision between labels the Operator creates, the Operator
	// labels take precedence.
	// +optional
	Labels map[string]string `json:"labels"`
	// Annotations are added to a resource. If there is a collision between annotations the Operator creates, the Operator
	// annotations take precedence.
	// +optional
	Annotations map[string]string `json:"annotations"`
}

type PodSpec struct {
	// Metadata is a subset of metav1.ObjectMeta applied to all pods.
	// +optional
	Metadata FullNodeMetadata `json:"metadata"`

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

// PersistentVolumeClaimSpec describes the common attributes of storage devices
// and allows a Source for provider-specific attributes
type PersistentVolumeClaimSpec struct {
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

// RolloutStrategy is an update strategy that can be shared between several Cosmos CRDs.
type RolloutStrategy struct {
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

type ChainConfig struct {
	// Genesis file chain-id.
	// +kubebuilder:validation:MinLength:=1
	ChainID string `json:"chainID"`

	// The network environment. Typically, mainnet, testnet, devnet, etc.
	// This field is immutable because it affects resource names.
	// +kubebuilder:validation:MinLength:=1
	Network string `json:"network"`

	// Binary name which runs commands. E.g. gaiad, junod, osmosisd
	// +kubebuilder:validation:MinLength:=1
	Binary string `json:"binary"`

	// Tendermint configuration applied to config.toml.
	// Although optional, it's highly recommended you configure this field.
	// +optional
	Tendermint TendermintConfig `json:"config"`

	// App configuration applied to app.toml.
	App SDKAppConfig `json:"app"`

	// One of trace|debug|info|warn|error|fatal|panic.
	// If not set, defaults to info.
	// +kubebuilder:validation:Enum:=trace;debug;info;warn;error;fatal;panic
	// +optional
	LogLevel *string `json:"logLevel"`

	// One of plain or json.
	// If not set, defaults to plain.
	// +kubebuilder:validation:Enum:=plain;json
	// +optional
	LogFormat *string `json:"logFormat"`

	// URL to genesis file to download from the internet.
	// Although this field is optional, you will almost always want to set it.
	// If not set, uses the genesis file created from the init subcommand. (This behavior may be desirable for new chains or testing.)
	// The operator detects and properly handles the following file extensions:
	// .json, .json.gz, .tar, .tar.gz, .tar.gzip, .zip
	// Use GenesisScript if the chain has an unconventional file format or genesis location.
	// +optional
	GenesisURL *string `json:"genesisURL"`

	// Specify shell (sh) script commands to properly download and save the genesis file.
	// Prefer GenesisURL if the file is in a conventional format.
	// The available shell commands are from docker image ghcr.io/strangelove-ventures/infra-toolkit, including wget and curl.
	// Save the file to env var $GENESIS_FILE.
	// E.g. curl https://url-to-genesis.com | jq '.genesis' > $GENESIS_FILE
	// Takes precedence over GenesisURL.
	// Hint: Use "set -eux" in your script.
	// Available env vars:
	// $HOME: The home directory.
	// $GENESIS_FILE: The location of the final genesis file.
	// $CONFIG_DIR: The location of the config dir that houses the genesis file. Used for extracting from archives. The archive must have a single file called "genesis.json".
	// +optional
	GenesisScript *string `json:"genesisScript"`

	// Skip x/crisis invariants check on startup.
	// +optional
	SkipInvariants bool `json:"skipInvariants"`

	// URL for a snapshot archive to download from the internet.
	// Unarchiving the snapshot populates the data directory.
	// Although this field is optional, you will almost always want to set it.
	// The operator detects and properly handles the following file extensions:
	// .tar, .tar.gz, .tar.gzip, .tar.lz4
	// Use SnapshotScript if the snapshot archive is unconventional or requires special handling.
	// +optional
	SnapshotURL *string `json:"snapshotURL"`

	// Specify shell (sh) script commands to properly download and process a snapshot archive.
	// Prefer SnapshotURL if possible.
	// The available shell commands are from docker image ghcr.io/strangelove-ventures/infra-toolkit, including wget and curl.
	// Save the file to env var $GENESIS_FILE.
	// Takes precedence over SnapshotURL.
	// Hint: Use "set -eux" in your script.
	// Available env vars:
	// $HOME: The user's home directory.
	// $CHAIN_HOME: The home directory for the chain, aka: --home flag
	// $DATA_DIR: The directory for the database files.
	// +optional
	SnapshotScript *string `json:"snapshotScript"`
}

// TendermintConfig configures the tendermint config.toml.
type TendermintConfig struct {
	// Comma delimited list of p2p nodes in <ID>@<IP>:<PORT> format to keep persistent p2p connections.
	// See https://docs.tendermint.com/master/spec/p2p/peer.html and
	// https://docs.tendermint.com/master/spec/p3p/config.html#persistent-peers.
	// +kubebuilder:validation:MinLength:=1
	// +optional
	PersistentPeers string `json:"peers"`

	// Comma delimited list of p2p seed nodes in <ID>@<IP>:<PORT> format.
	// See https://docs.tendermint.com/master/spec/p2p/config.html#seeds and
	// https://docs.tendermint.com/master/spec/p2p/node.html#seeds.
	// +kubebuilder:validation:MinLength:=1
	// +optional
	Seeds string `json:"seeds"`

	// p2p maximum number of inbound peers.
	// If unset, defaults to 20.
	// +kubebuilder:validation:Minimum:=1
	// +optional
	MaxInboundPeers *int32 `json:"maxInboundPeers"`

	// p2p maximum number of outbound peers.
	// If unset, defaults to 20.
	// +kubebuilder:validation:Minimum:=1
	// +optional
	MaxOutboundPeers *int32 `json:"maxOutboundPeers"`

	// rpc list of origins a cross-domain request can be executed from.
	// Default value '[]' disables cors support.
	// Use '["*"]' to allow any origin.
	// +optional
	CorsAllowedOrigins []string `json:"corsAllowedOrigins"`

	// Custom tendermint config toml.
	// Values entered here take precedence over all other configuration.
	// Must be valid toml.
	// Important: all keys must be "snake_case" which differs from app.toml.
	// +optional
	TomlOverrides *string `json:"overrides"`
}

// SDKAppConfig configures the cosmos sdk application app.toml.
type SDKAppConfig struct {
	// The minimum gas prices a validator is willing to accept for processing a
	// transaction. A transaction's fees must meet the minimum of any denomination
	// specified in this config (e.g. 0.25token1;0.0001token2).
	// +kubebuilder:validation:MinLength:=1
	MinGasPrice string `json:"minGasPrice"`

	// Defines if CORS should be enabled for the API (unsafe - use it at your own risk).
	// +optional
	APIEnableUnsafeCORS bool `json:"apiEnableUnsafeCORS"`

	// Defines if CORS should be enabled for grpc-web (unsafe - use it at your own risk).
	// +optional
	GRPCWebEnableUnsafeCORS bool `json:"grpcWebEnableUnsafeCORS"`

	// Controls pruning settings. i.e. How much data to keep on disk.
	// If not set, defaults to "default" pruning strategy.
	// +optional
	Pruning *Pruning `json:"pruning"`

	// If set, block height at which to gracefully halt the chain and shutdown the node.
	// Useful for testing or upgrades.
	// +kubebuilder:validation:Minimum:=1
	// +optional
	HaltHeight *uint64 `json:"haltHeight"`

	// Custom app config toml.
	// Values entered here take precedence over all other configuration.
	// Must be valid toml.
	// Important: all keys must be "kebab-case" which differs from config.toml.
	// +optional
	TomlOverrides *string `json:"overrides"`
}

// Pruning controls the pruning settings.
type Pruning struct {
	// One of default|nothing|everything|custom.
	// default: the last 100 states are kept in addition to every 500th state; pruning at 10 block intervals.
	// nothing: all historic states will be saved, nothing will be deleted (i.e. archiving node).
	// everything: all saved states will be deleted, storing only the current state; pruning at 10 block intervals.
	// custom: allow pruning options to be manually specified through Interval, KeepEvery, KeepRecent.
	// +kubebuilder:validation:Enum:=default;nothing;everything;custom
	Strategy PruningStrategy `json:"strategy"`

	// Bock height interval at which pruned heights are removed from disk (ignored if pruning is not 'custom').
	// If not set, defaults to 0.
	// +optional
	Interval *uint32 `json:"interval"`

	// Offset heights to keep on disk after 'keep-every' (ignored if pruning is not 'custom')
	// Often, setting this to 0 is appropriate.
	// If not set, defaults to 0.
	// +optional
	KeepEvery *uint32 `json:"keepEvery"`

	// Number of recent block heights to keep on disk (ignored if pruning is not 'custom')
	// If not set, defaults to 0.
	// +optional
	KeepRecent *uint32 `json:"keepRecent"`
}

// PruningStrategy control pruning.
type PruningStrategy string

const (
	PruningDefault    PruningStrategy = "default"
	PruningNothing    PruningStrategy = "nothing"
	PruningEverything PruningStrategy = "everything"
	PruningCustom     PruningStrategy = "custom"
)

type ServiceSpec struct {
	// Maximum number of p2p services to create for tendermint peer exchange.
	// The public endpoint is set as the "p2p.external_address" in the tendermint config.toml.
	// If not set, defaults to 3.
	// +kubebuilder:validation:Minimum:=0
	// +optional
	MaxP2PExternalAddresses *int32 `json:"maxP2PExternalAddresses"`

	// Overrides for the single RPC service.
	// +optional
	RPCTemplate RPCServiceSpec `json:"rpcTemplate"`
}

// RPCServiceSpec allows some overrides for the created, single RPC service.
type RPCServiceSpec struct {
	// Added to the single RPC service annotations. Some cloud providers require special annotations.
	// +optional
	Annotations map[string]string `json:"annotations"`

	// Describes ingress methods for a service.
	// If not set, defaults to "ClusterIP".
	// +kubebuilder:validation:Enum:=ClusterIP;NodePort;LoadBalancer;ExternalName
	// +optional
	Type *corev1.ServiceType `json:"type"`

	// Sets endpoint and routing behavior.
	// See: https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#caveats-and-limitations-when-preserving-source-ips
	// If not set, defaults to "Cluster".
	// +kubebuilder:validation:Enum:=Cluster;Local
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CosmosFullNode is the Schema for the cosmosfullnodes API
type CosmosFullNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FullNodeSpec   `json:"spec,omitempty"`
	Status FullNodeStatus `json:"status,omitempty"`
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
