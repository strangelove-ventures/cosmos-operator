package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	policyv1 "k8s.io/api/policy/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PodDisruptionBudgetName(crd *cosmosv1.CosmosFullNode) string {
	return appName(crd) + "-vc-pdb"
}

// BuildPodDisruptionBudget returns a list of pdbs given the crd.
//
// Creates a single pdb for the version check.
func BuildPodDisruptionBudget(crd *cosmosv1.CosmosFullNode) []diff.Resource[*policyv1.PodDisruptionBudget] {
	diffpdb := make([]diff.Resource[*policyv1.PodDisruptionBudget], 1)
	pdb := policyv1.PodDisruptionBudget{
		TypeMeta: v1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      PodDisruptionBudgetName(crd),
			Namespace: crd.Namespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: crd.Spec.PodDisruptionBudget.MaxUnavailable,
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					kube.ControllerLabel: "cosmos-operator",
					kube.NameLabel:       appName(crd),
				},
			},
		},
	}

	pdb.Labels = defaultLabels(crd, kube.ComponentLabel, "vc")

	diffpdb[0] = diff.Adapt(&pdb, 0)

	return diffpdb
}
