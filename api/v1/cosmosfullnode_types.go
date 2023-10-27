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

func init() {
	SchemeBuilder.Register(&CosmosFullNode{}, &CosmosFullNodeList{})
}

// CosmosFullNodeController is the canonical controller name.
const CosmosFullNodeController = "CosmosFullNode"

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

	// Different flavors of the fullnode's configuration.
	// 'Sentry' configures the fullnode as a validator sentry, requiring a remote signer such as Horcrux or TMKMS.
	// The remote signer is out of scope for the operator and must be deployed separately. Each pod exposes a privval port
	// for use with the remote signer.
	// If not set, configures node for RPC.
	// +kubebuilder:validation:Enum:=FullNode;Sentry
	// +optional
	Type FullNodeType `json:"type"`

	// Blockchain-specific configuration.
	ChainSpec ChainSpec `json:"chain"`

	// Template applied to all pods.
	// Creates 1 pod per replica.
	PodTemplate PodSpec `json:"podTemplate"`

	// How to scale pods when performing an update.
	// +optional
	RolloutStrategy RolloutStrategy `json:"strategy"`

	// Will be used to create a stand-alone PVC to provision the volume.
	// One PVC per replica mapped and mounted to a corresponding pod.
	VolumeClaimTemplate PersistentVolumeClaimSpec `json:"volumeClaimTemplate"`

	// Determines how to handle PVCs when pods are scaled down.
	// One of 'Retain' or 'Delete'.
	// If 'Delete', PVCs are deleted if pods are scaled down.
	// If 'Retain', PVCs are not deleted. The admin must delete manually or are deleted if the CRD is deleted.
	// If not set, defaults to 'Delete'.
	// +kubebuilder:validation:Enum:=Retain;Delete
	// +optional
	RetentionPolicy *RetentionPolicy `json:"volumeRetentionPolicy"`

	// Configure Operator created services. A singe rpc service is created for load balancing api, grpc, rpc, etc. requests.
	// This allows a k8s admin to use the service in an Ingress, for example.
	// Additionally, multiple p2p services are created for CometBFT peer exchange.
	// +optional
	Service ServiceSpec `json:"service"`

	// Allows overriding an instance on a case-by-case basis. An instance is a pod/pvc combo with an ordinal.
	// Key must be the name of the pod including the ordinal suffix.
	// Example: cosmos-1
	// Used for debugging.
	// +optional
	InstanceOverrides map[string]InstanceOverridesSpec `json:"instanceOverrides"`

	// Strategies for automatic recovery of faults and errors.
	// Managed by a separate controller, SelfHealingController, in an effort to reduce
	// complexity of the CosmosFullNodeController.
	// +optional
	SelfHeal *SelfHealSpec `json:"selfHeal"`
}

type FullNodeType string

const (
	FullNode FullNodeType = "FullNode"
	Sentry   FullNodeType = "Sentry"
)

// FullNodeStatus defines the observed state of CosmosFullNode
type FullNodeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration"`

	// The current phase of the fullnode deployment.
	// "Progressing" means the deployment is under way.
	// "Complete" means the deployment is complete and reconciliation is finished.
	// "WaitingForP2PServices" means the deployment is complete but the p2p services are not yet ready.
	// "Error" means an unrecoverable error occurred, which needs human intervention.
	Phase FullNodePhase `json:"phase"`

	// A generic message for the user. May contain errors.
	// +optional
	StatusMessage *string `json:"status"`

	// Set by the ScheduledVolumeSnapshotController. Used to signal the CosmosFullNode to modify its
	// resources during VolumeSnapshot creation.
	// Map key is the source ScheduledVolumeSnapshot CRD that created the status.
	// +optional
	// +mapType:=granular
	ScheduledSnapshotStatus map[string]FullNodeSnapshotStatus `json:"scheduledSnapshotStatus"`

	// Status set by the SelfHealing controller.
	// +optional
	SelfHealing SelfHealingStatus `json:"selfHealing,omitempty"`

	// Persistent peer addresses.
	// +optional
	Peers []string `json:"peers"`

	// Current sync information. Collected every 60s.
	// +optional
	SyncInfo map[string]*SyncInfoPodStatus `json:"sync,omitempty"`

	// Latest Height information. collected when node starts up and when RPC is successfully queried.
	// +optional
	Height map[string]uint64 `json:"height,omitempty"`
}

type SyncInfoPodStatus struct {
	// When consensus information was fetched.
	Timestamp metav1.Time `json:"timestamp"`
	// Latest height if no error encountered.
	// +optional
	Height *uint64 `json:"height,omitempty"`
	// If the pod reports itself as in sync with chain tip.
	// +optional
	InSync *bool `json:"inSync,omitempty"`
	// Error message if unable to fetch consensus state.
	// +optional
	Error *string `json:"error,omitempty"`
}

type FullNodeSnapshotStatus struct {
	// Which pod name to temporarily delete. Indicates a ScheduledVolumeSnapshot is taking place. For optimal data
	// integrity, pod is temporarily removed so PVC does not have any processes writing to it.
	PodCandidate string `json:"podCandidate"`
}

type FullNodePhase string

const (
	FullNodePhaseCompete        FullNodePhase = "Complete"
	FullNodePhaseError          FullNodePhase = "Error"
	FullNodePhaseP2PServices    FullNodePhase = "WaitingForP2PServices"
	FullNodePhaseProgressing    FullNodePhase = "Progressing"
	FullNodePhaseTransientError FullNodePhase = "TransientError"
)

// Metadata is a subset of k8s object metadata.
type Metadata struct {
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
	Metadata Metadata `json:"metadata"`

	// Image is the docker reference in "repository:tag" format. E.g. busybox:latest.
	// This is for the main container running the chain process.
	// Note: for granular control over which images are applied at certain block heights,
	// use spec.chain.versions instead.
	// +kubebuilder:validation:MinLength:=1
	// +optional
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

	// Configure probes for the pods managed by the controller.
	// +optional
	Probes FullNodeProbesSpec `json:"probes"`

	// List of volumes that can be mounted by containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// A strategic merge patch is applied to the default volumes created by the controller.
	// Take extreme caution when using this feature. Use only for critical bugs.
	// Some chains do not follow conventions or best practices, so this serves as an "escape hatch" for the user
	// at the cost of maintainability.
	// +optional
	Volumes []corev1.Volume `json:"volumes"`

	// List of initialization containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
	// A strategic merge patch is applied to the default init containers created by the controller.
	// Take extreme caution when using this feature. Use only for critical bugs.
	// Some chains do not follow conventions or best practices, so this serves as an "escape hatch" for the user
	// at the cost of maintainability.
	// +optional
	InitContainers []corev1.Container `json:"initContainers"`

	// List of containers belonging to the pod.
	// A strategic merge patch is applied to the default containers created by the controller.
	// Take extreme caution when using this feature. Use only for critical bugs.
	// Some chains do not follow conventions or best practices, so this serves as an "escape hatch" for the user
	// at the cost of maintainability.
	// +optional
	Containers []corev1.Container `json:"containers"`
}

type FullNodeProbeStrategy string

const (
	FullNodeProbeStrategyNone FullNodeProbeStrategy = "None"
)

// FullNodeProbesSpec configures probes for created pods
type FullNodeProbesSpec struct {
	// Strategy controls the default probes added by the controller.
	// None = Do not add any probes. May be necessary for Sentries using a remote signer.
	// +kubebuilder:validation:Enum:=None
	// +optional
	Strategy FullNodeProbeStrategy `json:"strategy"`
}

// PersistentVolumeClaimSpec describes the common attributes of storage devices
// and allows a Source for provider-specific attributes
type PersistentVolumeClaimSpec struct {
	// Applied to all PVCs.
	// +optional
	Metadata Metadata `json:"metadata"`

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

	// Can be used to specify either:
	// * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot)
	// * An existing PVC (PersistentVolumeClaim)
	// If the provisioner or an external controller can support the specified data source,
	// it will create a new volume based on the contents of the specified data source.
	// If the AnyVolumeDataSource feature gate is enabled, this field will always have
	// the same contents as the DataSourceRef field.
	// If you choose an existing PVC, the PVC must be in the same availability zone.
	// +optional
	DataSource *corev1.TypedLocalObjectReference `json:"dataSource"`

	// If set, discovers and dynamically sets dataSource for the PVC on creation.
	// No effect if dataSource field set; that field takes precedence.
	// Configuring autoDataSource may help boostrap new replicas more quickly.
	// +optional
	AutoDataSource *AutoDataSource `json:"autoDataSource"`
}

type RetentionPolicy string

const (
	RetentionPolicyRetain RetentionPolicy = "Retain"
	RetentionPolicyDelete RetentionPolicy = "Delete"
)

type AutoDataSource struct {
	// If set, chooses the most recent VolumeSnapshot matching the selector to use as the PVC dataSource.
	// See ScheduledVolumeSnapshot for a means of creating periodic VolumeSnapshots.
	// The VolumeSnapshots must be in the same namespace as the CosmosFullNode.
	// If no VolumeSnapshots found, controller logs error and still creates PVC.
	// +optional
	VolumeSnapshotSelector map[string]string `json:"volumeSnapshotSelector"`

	// If true, the volume snapshot selector will make sure the PVC
	// is restored from a VolumeSnapshot on the same node.
	// This is useful if the VolumeSnapshots are local to the node, e.g. for topolvm.
	MatchInstance bool `json:"matchInstance"`
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

type ChainSpec struct {
	// Genesis file chain-id.
	// +kubebuilder:validation:MinLength:=1
	ChainID string `json:"chainID"`

	// The network environment. Typically, mainnet, testnet, devnet, etc.
	// +kubebuilder:validation:MinLength:=1
	Network string `json:"network"`

	// Binary name which runs commands. E.g. gaiad, junod, osmosisd
	// +kubebuilder:validation:MinLength:=1
	Binary string `json:"binary"`

	// The chain's home directory is where the chain's data and config is stored.
	// This should be a single folder. E.g. .gaia, .dydxprotocol, .osmosisd, etc.
	// Set via --home flag when running the binary.
	// If empty, defaults to "cosmos" which translates to `chain start --home /home/operator/cosmos`.
	// Historically, several chains do not respect the --home and save data outside --home which crashes the pods.
	// Therefore, this option was introduced to mitigate those edge cases, so that you can specify the home directory
	// to match the chain's default home dir.
	// +optional
	HomeDir string `json:"homeDir"`

	// CometBFT (formerly Tendermint) configuration applied to config.toml.
	// Although optional, it's highly recommended you configure this field.
	// +optional
	Comet CometConfig `json:"config"`

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

	// URL to address book file to download from the internet.
	// The operator detects and properly handles the following file extensions:
	// .json, .json.gz, .tar, .tar.gz, .tar.gzip, .zip
	// Use AddrbookScript if the chain has an unconventional file format or address book location.
	// +optional
	AddrbookURL *string `json:"addrbookURL"`

	// Specify shell (sh) script commands to properly download and save the address book file.
	// Prefer AddrbookURL if the file is in a conventional format.
	// The available shell commands are from docker image ghcr.io/strangelove-ventures/infra-toolkit, including wget and curl.
	// Save the file to env var $ADDRBOOK_FILE.
	// E.g. curl https://url-to-addrbook.com > $ADDRBOOK_FILE
	// Takes precedence over AddrbookURL.
	// Hint: Use "set -eux" in your script.
	// Available env vars:
	// $HOME: The home directory.
	// $ADDRBOOK_FILE: The location of the final address book file.
	// $CONFIG_DIR: The location of the config dir that houses the address book file. Used for extracting from archives. The archive must have a single file called "addrbook.json".
	// +optional
	AddrbookScript *string `json:"addrbookScript"`

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

	// If configured as a Sentry, invokes sleep command with this value before running chain start command.
	// Currently, requires the privval laddr to be available immediately without any retry.
	// This workaround gives time for the connection to be made to a remote signer.
	// If a Sentry and not set, defaults to 10.
	// If set to 0, omits injecting sleep command.
	// Assumes chain image has `sleep` in $PATH.
	// +kubebuilder:validation:Minimum:=0
	// +optional
	PrivvalSleepSeconds *int32 `json:"privvalSleepSeconds"`

	// DatabaseBackend must match in order to detect the block height
	// of the chain prior to starting in order to pick the correct image version.
	// options: goleveldb, rocksdb, pebbledb
	// Defaults to goleveldb.
	// +optional
	DatabaseBackend *string `json:"databaseBackend"`

	// Versions of the chain and which height they should be applied.
	// When provided, the operator will automatically upgrade the chain as it reaches the specified heights.
	// If not provided, the operator will not upgrade the chain, and will use the image specified in the pod spec.
	// +optional
	Versions []ChainVersion `json:"versions"`
}

type ChainVersion struct {
	// The block height when this version should be applied.
	UpgradeHeight uint64 `json:"height"`

	// The docker image for this version in "repository:tag" format. E.g. busybox:latest.
	Image string `json:"image"`

	// Determines if the node should forcefully halt at the upgrade height.
	// +optional
	SetHaltHeight bool `json:"setHaltHeight,omitempty"`
}

// CometConfig configures the config.toml.
type CometConfig struct {
	// Comma delimited list of p2p nodes in <ID>@<IP>:<PORT> format to keep persistent p2p connections.
	// +kubebuilder:validation:MinLength:=1
	// +optional
	PersistentPeers string `json:"peers"`

	// Comma delimited list of p2p seed nodes in <ID>@<IP>:<PORT> format.
	// +kubebuilder:validation:MinLength:=1
	// +optional
	Seeds string `json:"seeds"`

	// Comma delimited list of node/peer IDs to keep private (will not be gossiped to other peers)
	// +optional
	PrivatePeerIDs string `json:"privatePeerIDs"`

	// Comma delimited list of node/peer IDs, to which a connection will be (re)established ignoring any existing limits.
	// +optional
	UnconditionalPeerIDs string `json:"unconditionalPeerIDs"`

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

	// Customize config.toml.
	// Values entered here take precedence over all other configuration.
	// Must be valid toml.
	// Important: all keys must be "snake_case" which differs from app.toml.
	// WARNING: Overriding may clobber some values that the operator sets such as persistent_peers, private_peer_ids,
	// and unconditional_peer_ids. Use the dedicated fields for these values which will merge values.
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
	// +kubebuilder:validation:Minimum:=0
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

	// Defines the minimum block height offset from the current
	// block being committed, such that all blocks past this offset are pruned
	// from CometBFT. It is used as part of the process of determining the
	// ResponseCommit.RetainHeight value during ABCI Commit. A value of 0 indicates
	// that no blocks should be pruned.
	//
	// This configuration value is only responsible for pruning Comet blocks.
	// It has no bearing on application state pruning which is determined by the
	// "pruning-*" configurations.
	//
	// Note: CometBFT block pruning is dependent on this parameter in conjunction
	// with the unbonding (safety threshold) period, state pruning and state sync
	// snapshot parameters to determine the correct minimum value of
	// ResponseCommit.RetainHeight.
	//
	// If not set, defaults to 0.
	// +optional
	MinRetainBlocks *uint32 `json:"minRetainBlocks"`
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
	// Max number of external p2p services to create for CometBFT peer exchange.
	// The public endpoint is set as the "p2p.external_address" in the config.toml.
	// Controller creates p2p services for each pod so that every pod can peer with each other internally in the cluster.
	// This setting allows you to control the number of p2p services exposed for peers outside of the cluster to use.
	// If not set, defaults to 1.
	// +kubebuilder:validation:Minimum:=0
	// +optional
	MaxP2PExternalAddresses *int32 `json:"maxP2PExternalAddresses"`

	// Overrides for all P2P services that need external addresses.
	// +optional
	P2PTemplate ServiceOverridesSpec `json:"p2pTemplate"`

	// Overrides for the single RPC service.
	// +optional
	RPCTemplate ServiceOverridesSpec `json:"rpcTemplate"`
}

// ServiceOverridesSpec allows some overrides for the created, single RPC service.
type ServiceOverridesSpec struct {
	// +optional
	Metadata Metadata `json:"metadata"`

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

// InstanceOverridesSpec allows overriding an instance which is pod/pvc combo with an ordinal
type InstanceOverridesSpec struct {
	// Disables whole or part of the instance.
	// Used for scenarios like debugging or deleting the PVC and restoring from a dataSource.
	// Set to "Pod" to prevent controller from creating a pod for this instance, leaving the PVC.
	// Set to "All" to prevent the controller from managing a pod and pvc. Note, the PVC may not be deleted if
	// the RetainStrategy is set to "Retain". If you need to remove the PVC, delete manually.
	// +kubebuilder:validation:Enum:=Pod;All
	// +optional
	DisableStrategy *DisableStrategy `json:"disable"`

	// Overrides an individual instance's PVC.
	// +optional
	VolumeClaimTemplate *PersistentVolumeClaimSpec `json:"volumeClaimTemplate"`

	// Overrides an individual instance's Image.
	// +optional
	Image string `json:"image"`
}

type DisableStrategy string

const (
	DisableAll DisableStrategy = "All"
	DisablePod DisableStrategy = "Pod"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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
