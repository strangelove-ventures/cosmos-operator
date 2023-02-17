package v1

import "k8s.io/apimachinery/pkg/util/intstr"

// SelfHealingSpec is part of a CosmosFullNode but is managed by a separate controller, SelfHealingController.
// This is an effort to reduce complexity in the CosmosFullNodeController.
type SelfHealingSpec struct {
	// Determines when to destroy and recreate a replica (aka pod/pvc combo) that is crashlooping.
	// Occasionally, data may become corrupt and the chain exits and cannot restart.
	// This strategy only watches the pods' "node" containers running the `start` command.
	//
	// This pairs well with volumeClaimTemplate.autoDataSource and a ScheduledVolumeSnapshot resource.
	// With this pairing, a new PVC is created with a recent VolumeSnapshot.
	// Otherwise, ensure your snapshot, genesis, etc. creation are idempotent.
	// (e.g. chain.snapshotURL and chain.genesisURL have stable urls)
	//
	// +optional
	CrashLoopRecovery *CrashLoopRecovery `json:"crashLoopRecovery"`
}

type CrashLoopRecovery struct {
	// How many healthy pods are required to trigger destroying a crashlooping pod and pvc.
	// Set an integer or a percentage string such as 50%.
	// Example: If you set to 80% and there are 10 total pods, at least 8 must be healthy to trigger the recovery.
	// Fractional values are rounded down, but the minimum is 1.
	// It's not recommended to use this feature with only 1 replica.
	//
	// This setting attempts to minimize false positives in order to detect data corruption vs.
	// endless other reasons for unhealthy pods.
	// If the majority of pods are unhealthy, then there's probably something else wrong, and recreating
	// the pod and pvc will have no effect.
	HealthyThreshold intstr.IntOrString `json:"healthyThreshold"`

	// How many restarts to wait before destroying and recreating the unhealthy replica.
	// Defaults to 5.
	// +optional
	RestartThreshold int32 `json:"restartThreshold"`
}
