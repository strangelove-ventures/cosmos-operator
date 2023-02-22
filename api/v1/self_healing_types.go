package v1

// SelfHealingSpec is part of a CosmosFullNode but is managed by a separate controller, SelfHealingController.
// This is an effort to reduce complexity in the CosmosFullNodeController.
// The controller only modifies the CosmosFullNode's status subresource relying on the CosmosFullNodeController
// to reconcile appropriately.
type SelfHealingSpec struct {
	// Automatically increases PVC storage as they approach capacity.
	//
	// Your cluster must support and use the ExpandInUsePersistentVolumes feature gate. This allows volumes to
	// expand while a pod is attached to it, thus eliminating the need to restart pods.
	// If you cluster does not support ExpandInUsePersistentVolumes, you will manually need to restart pods.
	// +optional
	PVCAutoScaling *PVCAutoScalingSpec `json:"pvcAutoScaling"`
}

type PVCAutoScalingSpec struct {
	// The percentage of used disk space required to trigger scaling.
	// Example, if set to 80, autoscaling will not trigger until used space reaches >=80% of capacity.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	UsedSpacePercentage int32 `json:"usedSpacePercentage"`

	// How much to increase the PVC's capacity.
	// Either a percentage (e.g. 20%) or a resource storage quantity (e.g. 100Gi).
	//
	// If a percentage, the existing capacity increases by the percentage.
	// E.g. PVC of 100Gi capacity + Quantity of 20% increases disk to 120Gi.
	//
	// If a storage quantity (e.g. 100Gi), increases by that amount.
	Quantity string `json:"quantity"`

	// A resource storage quantity (e.g. 2000Gi).
	// When increasing PVC capacity reaches >= Maximum, autoscaling ceases.
	// Safeguards against storage quotas.
	// +optional
	Maximum string `json:"maximum"`
}
