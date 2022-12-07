package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
)

// ResetStatus is used at the beginning of the reconcile loop.
// It resets the crd's status to a fresh state.
func ResetStatus(crd *cosmosv1.CosmosFullNode) {
	crd.Status.ObservedGeneration = crd.Generation
	crd.Status.Phase = cosmosv1.FullNodePhaseProgressing
	crd.Status.StatusMessage = nil
}
