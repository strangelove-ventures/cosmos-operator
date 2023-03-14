package statefuljob

import (
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
)

func defaultLabels() map[string]string {
	return map[string]string{
		kube.ControllerLabel: "cosmos-operator",
		kube.ComponentLabel:  cosmosalpha.StatefulJobController,
	}
}
