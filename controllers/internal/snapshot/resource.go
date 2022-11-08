package snapshot

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
)

// ResourceName is the name of all resources created by the controller.
func ResourceName(crd *cosmosv1.HostedSnapshot) string {
	return kube.ToName("snapshot-" + crd.Name)
}
