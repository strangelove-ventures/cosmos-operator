package v1

import "k8s.io/apimachinery/pkg/util/intstr"

// SelfHealingSpec is part of a CosmosFullNode but is managed by a separate controller, SelfHealingController.
// This is an effort to reduce complexity in the CosmosFullNodeController.
type SelfHealingSpec struct {
	// Determines when to destroy and recreate a replica (aka instance, pod/pvc combo) that is failing.
	// Occasionally, data may become corrupt and the chain exits and cannot restart.
	// This strategy only watches the pods "node" container running the `start` command, and only for
	// pods that are crashlooping.
	//
	// This pairs well with volumeClaimTemplate.autoDataSource and a ScheduledVolumeSnapshot resource.
	// With this pairing, a new PVC is created with a recent VolumeSnapshot.
	// Otherwise, ensure your snapshot, genesis, etc. creation are idempotent.
	// (e.g. chain.snapshotURL and chain.genesisURL have stable urls)
	// +optional
	ReplicaFaultRecovery *ReplicaFaultRecoverySpec `json:"replicaFaultRecovery"`
}

type ReplicaFaultRecoverySpec struct {
	// How many healthy pods are required to trigger destroying a crashlooping pod and pvc.
	// The controller periodically inspects the status of all pods.
	// If the majority of pods are crashlooping, then there's probably something else wrong, and recreating
	// the pod and pvc will have no effect.
	// Set an integer or a percentage string such as 50%.
	// If the threshold is too high, defaults to watching for 1 unhealthy pod.
	HealthyThreshold intstr.IntOrString `json:"healthyThreshold"`

	// How many restarts to wait before destroying and recreating an unhealthy replica.
	// Defaults to 5.
	// +optional
	RestartThreshold int32 `json:"restartThreshold"`
}
