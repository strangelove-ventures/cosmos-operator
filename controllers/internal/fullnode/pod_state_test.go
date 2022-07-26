package fullnode

import (
	"strings"
	"testing"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodState(t *testing.T) {
	t.Parallel()

	crd := &cosmosv1.CosmosFullNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "agoric",
		},
		Spec: cosmosv1.CosmosFullNodeSpec{
			Replicas: 5,
			Image:    "busybox:latest",
		},
	}

	pods := PodState(crd)
	require.Len(t, pods, 5)

	want := strings.Split("agoric-0 agoric-1 agoric-2 agoric-3 agoric-4", " ")
	got := lo.Map(pods, func(pod *corev1.Pod, _ int) string { return pod.Name })
	require.Equal(t, want, got)

	pod := NewPodBuilder(crd).WithOrdinal(0).Build()
	require.Equal(t, pod, pods[0])
}
