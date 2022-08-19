package fullnode

import (
	"encoding/hex"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BuildServices returns a list of services given the crd.
func BuildServices(crd *cosmosv1.CosmosFullNode) []*corev1.Service {
	labels := defaultLabels(crd,
		kube.RevisionLabel, serviceRevisionHash(crd),
	)

	return []*corev1.Service{
		{
			TypeMeta: metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      appName(crd) + "-p2p",
				Namespace: crd.Namespace,
				Labels:    labels,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "p2p",
						Protocol:   corev1.ProtocolTCP,
						Port:       p2pPort,
						TargetPort: intstr.FromString("p2p"),
					},
				},
				Selector: SelectorLabels(crd),
				Type:     corev1.ServiceTypeLoadBalancer,
			},
		},
	}
}

// only requires update if the labels change
func serviceRevisionHash(crd *cosmosv1.CosmosFullNode) string {
	labels := lo.MapToSlice(defaultLabels(crd), func(v string, k string) string {
		return k + v
	})
	sort.Strings(labels)
	h := fnv.New32()
	_, err := h.Write([]byte(strings.Join(labels, "")))
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
