package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SelfHealingController is the canonical controller name.
const SelfHealingController = "SelfHealing"

// SelfHealSpec is part of a CosmosFullNode but is managed by a separate controller, SelfHealingReconciler.
// This is an effort to reduce complexity in the CosmosFullNodeReconciler.
// The controller only modifies the CosmosFullNode's status subresource relying on the CosmosFullNodeReconciler
// to reconcile appropriately.
type SelfHealSpec struct {
	// Automatically increases PVC storage as they approach capacity.
	//
	// Your cluster must support and use the ExpandInUsePersistentVolumes feature gate. This allows volumes to
	// expand while a pod is attached to it, thus eliminating the need to restart pods.
	// If you cluster does not support ExpandInUsePersistentVolumes, you will need to manually restart pods after
	// resizing is complete.
	// +optional
	PVCAutoScale *PVCAutoScaleSpec `json:"pvcAutoScale"`

	// Take action when a pod's height falls behind the max height of all pods AND still reports itself as in-sync.
	//
	// +optional
	HeightDriftMitigation *HeightDriftMitigationSpec `json:"heightDriftMitigation"`
}

type PVCAutoScaleSpec struct {
	// The percentage of used disk space required to trigger scaling.
	// Example, if set to 80, autoscaling will not trigger until used space reaches >=80% of capacity.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:MaxSize=100
	UsedSpacePercentage int32 `json:"usedSpacePercentage"`

	// How much to increase the PVC's capacity.
	// Either a percentage (e.g. 20%) or a resource storage quantity (e.g. 100Gi).
	//
	// If a percentage, the existing capacity increases by the percentage.
	// E.g. PVC of 100Gi capacity + IncreaseQuantity of 20% increases disk to 120Gi.
	//
	// If a storage quantity (e.g. 100Gi), increases by that amount.
	IncreaseQuantity string `json:"increaseQuantity"`

	// A resource storage quantity (e.g. 2000Gi).
	// When increasing PVC capacity reaches >= MaxSize, autoscaling ceases.
	// Safeguards against storage quotas and costs.
	// +optional
	MaxSize resource.Quantity `json:"maxSize"`
}

type HeightDriftMitigationSpec struct {
	// If pod's height falls behind the max height of all pods by this value or more AND the pod's RPC /status endpoint
	// reports itself as in-sync, the pod is deleted. The CosmosFullNodeController creates a new pod to replace it.
	// Pod deletion respects the CosmosFullNode.Spec.RolloutStrategy and will not delete more pods than set
	// by the strategy to prevent downtime.
	// This workaround is necessary to mitigate a bug in the Cosmos SDK and/or CometBFT where pods report themselves as
	// in-sync even though they can lag thousands of blocks behind the chain tip and cannot catch up.
	// A "rebooted" pod /status reports itself correctly and allows it to catch up to chain tip.
	// +kubebuilder:validation:Minimum:=1
	Threshold uint32 `json:"threshold"`
}

type SelfHealingStatus struct {
	// PVC auto-scaling status.
	// +optional
	PVCAutoScale map[string]*PVCAutoScaleStatus `json:"pvcAutoScaler"`
}

type PVCAutoScaleStatus struct {
	// The PVC size requested by the SelfHealing controller.
	RequestedSize resource.Quantity `json:"requestedSize"`
	// The timestamp the SelfHealing controller requested a PVC increase.
	RequestedAt metav1.Time `json:"requestedAt"`
}
