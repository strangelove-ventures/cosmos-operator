package statefuljob

import (
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
)

// ResourceName is the name of all resources created by the controller.
func ResourceName(crd *cosmosalpha.StatefulJob) string {
	return kube.ToName(crd.Name)
}
