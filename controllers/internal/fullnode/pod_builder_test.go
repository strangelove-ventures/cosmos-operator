package fullnode

import (
	"strings"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func defaultCRD() cosmosv1.CosmosFullNode {
	return cosmosv1.CosmosFullNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "osmosis",
			Namespace:       "test",
			ResourceVersion: "_resource_version_",
		},
		Spec: cosmosv1.CosmosFullNodeSpec{
			PodTemplate: cosmosv1.CosmosFullNodePodSpec{
				Image: "busybox:v1.2.3",
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("5"),
						corev1.ResourceMemory: resource.MustParse("5Gi"),
					},
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("500M"),
					},
				},
			},
		},
	}
}

func TestPodBuilder(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := cosmosv1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	t.Parallel()

	crd := defaultCRD()

	t.Run("happy path", func(t *testing.T) {
		builder := NewPodBuilder(&crd)
		pod := builder.WithOrdinal(5).Build()

		require.Equal(t, "Pod", pod.Kind)
		require.Equal(t, "v1", pod.APIVersion)

		require.Equal(t, "test", pod.Namespace)
		require.Equal(t, "osmosis-fullnode-5", pod.Name)
		wantLabels := map[string]string{
			"cosmosfullnode.cosmos.strange.love/chain-name": "osmosis",
			"app.kubernetes.io/instance":                    "osmosis-fullnode-5",
			"app.kubernetes.io/created-by":                  "cosmosfullnode",
			"app.kubernetes.io/name":                        "osmosis-fullnode",
			"app.kubernetes.io/version":                     "v1.2.3",
		}
		require.Equal(t, wantLabels, pod.Labels)
		require.EqualValues(t, 30, *pod.Spec.TerminationGracePeriodSeconds)

		require.NotEmpty(t, pod.Annotations["cosmosfullnode.cosmos.strange.love/resource-revision"])
		// The fuzz test below tests this property.
		delete(pod.Annotations, revisionAnnotation)

		wantAnnotations := map[string]string{
			"cosmosfullnode.cosmos.strange.love/ordinal": "5",
			// TODO (nix - 8/2/22) Prom metrics here
		}
		require.Equal(t, wantAnnotations, pod.Annotations)

		require.Len(t, pod.Spec.Containers, 1)

		lastContainer := pod.Spec.Containers[len(pod.Spec.Containers)-1]
		require.Equal(t, "osmosis", lastContainer.Name)
		require.Equal(t, "busybox:v1.2.3", lastContainer.Image)
		require.Equal(t, corev1.PullIfNotPresent, lastContainer.ImagePullPolicy)
		require.Equal(t, crd.Spec.PodTemplate.Resources, lastContainer.Resources)

		// Test we don't share or leak data per invocation.
		pod = builder.Build()
		require.Empty(t, pod.Name)

		pod = builder.WithOrdinal(123).Build()
		require.Equal(t, "osmosis-fullnode-123", pod.Name)
	})

	t.Run("happy path - ports", func(t *testing.T) {
		pod := NewPodBuilder(&crd).Build()
		ports := pod.Spec.Containers[len(pod.Spec.Containers)-1].Ports

		require.Len(t, ports, 7)

		for i, tt := range []struct {
			Name string
			Port int32
		}{
			{"api", 1317},
			{"rosetta", 8080},
			{"grpc", 9090},
			{"prometheus", 26660},
			{"p2p", 26656},
			{"rpc", 26657},
			{"web", 9091},
		} {
			port := ports[i]
			require.Equal(t, tt.Name, port.Name, tt)
			require.Equal(t, corev1.ProtocolTCP, port.Protocol)
			require.Equal(t, tt.Port, port.ContainerPort)
			require.Zero(t, port.HostPort)
		}
	})

	t.Run("long name", func(t *testing.T) {
		longCrd := crd.DeepCopy()
		longCrd.Name = strings.Repeat("a", 253)

		builder := NewPodBuilder(longCrd)
		pod := builder.WithOrdinal(125).Build()

		require.Len(t, pod.Name, 253)
		require.Regexp(t, `a.*-fullnode-125`, pod.Name)

		wantLabels := map[string]string{
			"cosmosfullnode.cosmos.strange.love/chain-name": strings.Repeat("a", 63),
			"app.kubernetes.io/instance":                    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-fullnode-125",
			"app.kubernetes.io/created-by":                  "cosmosfullnode",
			"app.kubernetes.io/name":                        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-fullnode",
			"app.kubernetes.io/version":                     "v1.2.3",
		}
		require.Equal(t, wantLabels, pod.Labels)
	})
}

//nolint:stylecheck // Underscore in func name is acceptable for tests.
func FuzzPodBuilder_Build(f *testing.F) {
	crd := defaultCRD()
	f.Add("busybox:latest")
	f.Fuzz(func(t *testing.T, image string) {
		crd.Spec.PodTemplate.Image = image
		pod1 := NewPodBuilder(&crd).Build()
		pod2 := NewPodBuilder(&crd).Build()

		require.NotEmpty(t, pod1.Annotations[revisionAnnotation], image)
		require.NotEmpty(t, pod2.Annotations[revisionAnnotation], image)

		require.Equal(t, pod1.Annotations[revisionAnnotation], pod2.Annotations[revisionAnnotation], image)
	})
}
