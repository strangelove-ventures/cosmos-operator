package fullnode

import (
	"strings"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
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
			PodTemplate: cosmosv1.CosmosPodSpec{
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

	t.Run("happy path - critical fields", func(t *testing.T) {
		builder := NewPodBuilder(&crd)
		pod := builder.WithOrdinal(5).Build()

		require.Equal(t, "Pod", pod.Kind)
		require.Equal(t, "v1", pod.APIVersion)

		require.Equal(t, "test", pod.Namespace)
		require.Equal(t, "osmosis-fullnode-5", pod.Name)

		require.NotEmpty(t, pod.Labels["cosmosfullnode.cosmos.strange.love/resource-revision"])
		// The fuzz test below tests this property.
		delete(pod.Labels, revisionLabel)
		wantLabels := map[string]string{
			"app.kubernetes.io/instance":   "osmosis-fullnode-5",
			"app.kubernetes.io/created-by": "cosmosfullnode",
			"app.kubernetes.io/name":       "osmosis-fullnode",
			"app.kubernetes.io/version":    "v1.2.3",
		}
		require.Equal(t, wantLabels, pod.Labels)

		require.EqualValues(t, 30, *pod.Spec.TerminationGracePeriodSeconds)

		wantAnnotations := map[string]string{
			"cosmosfullnode.cosmos.strange.love/ordinal": "5",
			// TODO (nix - 8/2/22) Prom metrics here
		}
		require.Equal(t, wantAnnotations, pod.Annotations)

		require.Len(t, pod.Spec.Containers, 1)

		lastContainer := pod.Spec.Containers[len(pod.Spec.Containers)-1]
		require.Equal(t, "osmosis", lastContainer.Name)
		require.Equal(t, "busybox:v1.2.3", lastContainer.Image)
		require.Empty(t, lastContainer.ImagePullPolicy)
		require.Equal(t, crd.Spec.PodTemplate.Resources, lastContainer.Resources)
		require.NotEmpty(t, lastContainer.VolumeMounts) // TODO (nix - 8/12/22) Better assertion once we know what container needs.

		vols := pod.Spec.Volumes
		require.Len(t, vols, 1)
		require.Equal(t, "vol-osmosis-fullnode-5", vols[0].Name)
		require.Equal(t, "pvc-osmosis-fullnode-5", vols[0].PersistentVolumeClaim.ClaimName)

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

	t.Run("default affinity", func(t *testing.T) {
		pod := NewPodBuilder(&crd).WithOrdinal(1).Build()

		want := &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 1,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app.kubernetes.io/name": "osmosis-fullnode"},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		}

		require.Equal(t, want, pod.Spec.Affinity)
	})

	t.Run("happy path - optional fields", func(t *testing.T) {
		optCrd := crd.DeepCopy()

		optCrd.Spec.PodTemplate.Metadata.Labels = map[string]string{"custom": "label", kube.NameLabel: "should not see me"}
		optCrd.Spec.PodTemplate.Metadata.Annotations = map[string]string{"custom": "annotation", OrdinalAnnotation: "should not see me"}

		optCrd.Spec.PodTemplate.Affinity = &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "affinity1"}},
			},
		}
		optCrd.Spec.PodTemplate.ImagePullPolicy = corev1.PullAlways
		optCrd.Spec.PodTemplate.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "pullSecrets"}}
		optCrd.Spec.PodTemplate.NodeSelector = map[string]string{"node": "test"}
		optCrd.Spec.PodTemplate.Tolerations = []corev1.Toleration{{Key: "toleration1"}}
		optCrd.Spec.PodTemplate.PriorityClassName = "priority1"
		optCrd.Spec.PodTemplate.Priority = ptr(int32(55))
		optCrd.Spec.PodTemplate.TerminationGracePeriodSeconds = ptr(int64(40))

		builder := NewPodBuilder(optCrd)
		pod := builder.WithOrdinal(9).Build()

		require.Equal(t, "label", pod.Labels["custom"])
		// Operator label takes precedence.
		require.Equal(t, "osmosis-fullnode", pod.Labels[kube.NameLabel])

		require.Equal(t, "annotation", pod.Annotations["custom"])
		// Operator label takes precedence.
		require.Equal(t, "9", pod.Annotations[OrdinalAnnotation])

		require.Equal(t, optCrd.Spec.PodTemplate.Affinity, pod.Spec.Affinity)
		require.Equal(t, optCrd.Spec.PodTemplate.Tolerations, pod.Spec.Tolerations)
		require.EqualValues(t, 40, *optCrd.Spec.PodTemplate.TerminationGracePeriodSeconds)
		require.Equal(t, optCrd.Spec.PodTemplate.NodeSelector, pod.Spec.NodeSelector)

		require.Equal(t, "priority1", pod.Spec.PriorityClassName)
		require.EqualValues(t, 55, *pod.Spec.Priority)
		require.Equal(t, optCrd.Spec.PodTemplate.ImagePullSecrets, pod.Spec.ImagePullSecrets)

		require.EqualValues(t, "Always", pod.Spec.Containers[0].ImagePullPolicy)
	})

	t.Run("long name", func(t *testing.T) {
		longCrd := crd.DeepCopy()
		longCrd.Name = strings.Repeat("a", 253)

		builder := NewPodBuilder(longCrd)
		pod := builder.WithOrdinal(125).Build()

		require.Regexp(t, `a.*-fullnode-125`, pod.Name)

		RequireValidMetadata(t, pod)
	})
}

func FuzzPodBuilderBuild(f *testing.F) {
	crd := defaultCRD()
	f.Add("busybox:latest", "cpu")
	f.Fuzz(func(t *testing.T, image, resourceName string) {
		crd.Spec.PodTemplate.Image = image
		crd.Spec.PodTemplate.Resources = corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceName(resourceName): resource.MustParse("1")},
		}
		pod1 := NewPodBuilder(&crd).Build()
		pod2 := NewPodBuilder(&crd).Build()

		require.NotEmpty(t, pod1.Labels[revisionLabel], image)
		require.NotEmpty(t, pod2.Labels[revisionLabel], image)

		require.Equal(t, pod1.Labels[revisionLabel], pod2.Labels[revisionLabel], image)

		crd.Spec.PodTemplate.Resources = corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceName(resourceName): resource.MustParse("2")}, // Changed value here.
		}
		pod3 := NewPodBuilder(&crd).Build()

		require.NotEqual(t, pod1.Labels[revisionLabel], pod3.Labels[revisionLabel])
	})
}
