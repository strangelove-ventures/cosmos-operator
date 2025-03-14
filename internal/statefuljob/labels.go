package statefuljob

import (
	"github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
)

func defaultLabels() map[string]string {
	var _ = v1alpha1.StatefulJobController

	return map[string]string{
		kube.ControllerLabel: "cosmos-operator",
		kube.ComponentLabel:  v1alpha1.StatefulJobController,
	}
}
