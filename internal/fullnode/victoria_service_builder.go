package fullnode

import (
	victoriametricsv1beta1 "github.com/VictoriaMetrics/operator/api/v1beta1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildVMServiceScrape returns a list of VMServiceScrape given the crd.
// Creates a single VMServiceScrape for the fullnode.
func BuildVMServiceScrape(crd *cosmosv1.CosmosFullNode) []diff.Resource[*victoriametricsv1beta1.VMServiceScrape] {
	diffvs := make([]diff.Resource[*victoriametricsv1beta1.VMServiceScrape], 1)
	vs := victoriametricsv1beta1.VMServiceScrape{
		TypeMeta: v1.TypeMeta{
			Kind:       "VMServiceScrape",
			APIVersion: "v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      appName(crd),
			Namespace: crd.Namespace,
		},
		Spec: victoriametricsv1beta1.VMServiceScrapeSpec{
			Selector: v1.LabelSelector{
				MatchLabels: map[string]string{
					kube.ControllerLabel: "cosmos-operator",
					kube.NameLabel:       appName(crd),
					kube.ComponentLabel:  "rpc",
				},
			},
			Endpoints: []victoriametricsv1beta1.Endpoint{
				{
					Interval: "15s",
					Port:     "prometheus",
					Path:     "/metrics",
				},
			},
		},
	}

	vs.Labels = defaultLabels(crd, kube.ComponentLabel, "vc")
	diffvs[0] = diff.Adapt(&vs, 0)

	return diffvs
}
