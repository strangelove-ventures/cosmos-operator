package fullnode

import (
	"strings"
	"testing"

	"github.com/samber/lo"
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
			ChainConfig: cosmosv1.CosmosChainConfig{Network: "mainnet"},
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

	t.Run("happy path - critical fields", func(t *testing.T) {
		crd := defaultCRD()
		builder := NewPodBuilder(&crd)
		pod := builder.WithOrdinal(5).Build()

		require.Equal(t, "Pod", pod.Kind)
		require.Equal(t, "v1", pod.APIVersion)

		require.Equal(t, "test", pod.Namespace)
		require.Equal(t, "osmosis-mainnet-fullnode-5", pod.Name)

		require.NotEmpty(t, pod.Labels["app.kubernetes.io/revision"])
		// The fuzz test below tests this property.
		delete(pod.Labels, kube.RevisionLabel)
		wantLabels := map[string]string{
			"app.kubernetes.io/instance":   "osmosis-mainnet-fullnode-5",
			"app.kubernetes.io/created-by": "cosmosfullnode",
			"app.kubernetes.io/name":       "osmosis-mainnet-fullnode",
			"app.kubernetes.io/version":    "v1.2.3",
			"cosmos.strange.love/network":  "mainnet",
		}
		require.Equal(t, wantLabels, pod.Labels)

		require.EqualValues(t, 30, *pod.Spec.TerminationGracePeriodSeconds)

		wantAnnotations := map[string]string{
			"app.kubernetes.io/ordinal": "5",
			// TODO (nix - 8/2/22) Prom metrics here
		}
		require.Equal(t, wantAnnotations, pod.Annotations)

		sc := pod.Spec.SecurityContext
		require.EqualValues(t, 1025, *sc.RunAsUser)
		require.EqualValues(t, 1025, *sc.RunAsGroup)
		require.EqualValues(t, 1025, *sc.FSGroup)
		require.EqualValues(t, "OnRootMismatch", *sc.FSGroupChangePolicy)
		require.True(t, *sc.RunAsNonRoot)

		// Test we don't share or leak data per invocation.
		pod = builder.Build()
		require.Empty(t, pod.Name)

		pod = builder.WithOrdinal(123).Build()
		require.Equal(t, "osmosis-mainnet-fullnode-123", pod.Name)
	})

	t.Run("happy path - ports", func(t *testing.T) {
		crd := defaultCRD()
		pod := NewPodBuilder(&crd).Build()
		ports := pod.Spec.Containers[0].Ports

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

	t.Run("happy path - optional fields", func(t *testing.T) {
		optCrd := defaultCRD()

		optCrd.Spec.PodTemplate.Metadata.Labels = map[string]string{"custom": "label", kube.NameLabel: "should not see me"}
		optCrd.Spec.PodTemplate.Metadata.Annotations = map[string]string{"custom": "annotation", kube.OrdinalAnnotation: "should not see me"}

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

		builder := NewPodBuilder(&optCrd)
		pod := builder.WithOrdinal(9).Build()

		require.Equal(t, "label", pod.Labels["custom"])
		// Operator label takes precedence.
		require.Equal(t, "osmosis-mainnet-fullnode", pod.Labels[kube.NameLabel])

		require.Equal(t, "annotation", pod.Annotations["custom"])
		// Operator label takes precedence.
		require.Equal(t, "9", pod.Annotations[kube.OrdinalAnnotation])

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
		longCrd := defaultCRD()
		longCrd.Name = strings.Repeat("a", 253)

		builder := NewPodBuilder(&longCrd)
		pod := builder.WithOrdinal(125).Build()

		require.Regexp(t, `a.*-mainnet-fullnode-125`, pod.Name)

		RequireValidMetadata(t, pod)
	})

	t.Run("containers", func(t *testing.T) {
		crd := defaultCRD()
		const wantWrkDir = "/home/operator"
		crd.Spec.ChainConfig.Binary = "osmosisd"
		crd.Spec.ChainConfig.SnapshotURL = ptr("https://example.com/snapshot.tar")
		builder := NewPodBuilder(&crd)
		pod := builder.WithOrdinal(6).Build()

		require.Len(t, pod.Spec.Containers, 1)

		container := pod.Spec.Containers[0]
		require.Equal(t, "osmosis", container.Name)
		require.Empty(t, container.ImagePullPolicy)
		require.Equal(t, crd.Spec.PodTemplate.Resources, container.Resources)
		require.Equal(t, wantWrkDir, container.WorkingDir)

		require.Equal(t, container.Env[0].Name, "HOME")
		require.Equal(t, container.Env[0].Value, "/home/operator")
		require.Equal(t, container.Env[1].Name, "CHAIN_HOME")
		require.Equal(t, container.Env[1].Value, "/home/operator/cosmos")
		require.Equal(t, container.Env[2].Name, "GENESIS_FILE")
		require.Equal(t, container.Env[2].Value, "/home/operator/cosmos/config/genesis.json")
		require.Equal(t, container.Env[3].Name, "CONFIG_DIR")
		require.Equal(t, container.Env[3].Value, "/home/operator/cosmos/config")
		require.Equal(t, container.Env[4].Name, "DATA_DIR")
		require.Equal(t, container.Env[4].Value, "/home/operator/cosmos/data")
		require.Equal(t, envVars, container.Env)

		require.Len(t, lo.Map(pod.Spec.InitContainers, func(c corev1.Container, _ int) string { return c.Name }), 4)

		chown := pod.Spec.InitContainers[0]
		// Can't have security context for chown to succeed.
		require.Nil(t, chown.SecurityContext)
		require.Equal(t, wantWrkDir, chown.WorkingDir)
		require.Equal(t, envVars, chown.Env)

		for _, c := range pod.Spec.InitContainers {
			require.Equal(t, envVars, container.Env, c.Name)
			require.Equal(t, wantWrkDir, c.WorkingDir)
		}

		initCont := pod.Spec.InitContainers[0]
		require.Contains(t, initCont.Args[1], `osmosisd init osmosis-mainnet-fullnode-6 --home "$CHAIN_HOME"`)
		require.Contains(t, initCont.Args[1], `osmosisd init osmosis-mainnet-fullnode-6 --home "$HOME/.tmp"`)

		mergeConfig := pod.Spec.InitContainers[2]
		// The order of config-merge arguments is important. Rightmost takes precedence.
		require.Contains(t, mergeConfig.Args[1], `config-merge -f toml "$TMP_DIR/config.toml" "$OVERLAY_DIR/config-overlay.toml" > "$CONFIG_DIR/config.toml"`)
		require.Contains(t, mergeConfig.Args[1], `config-merge -f toml "$TMP_DIR/app.toml" "$OVERLAY_DIR/app-overlay.toml" > "$CONFIG_DIR/app.toml`)
	})

	t.Run("optional containers", func(t *testing.T) {
		crd := defaultCRD()
		pod := NewPodBuilder(&crd).WithOrdinal(0).Build()

		require.Equal(t, 3, len(pod.Spec.InitContainers))
	})

	t.Run("volumes", func(t *testing.T) {
		crd := defaultCRD()
		builder := NewPodBuilder(&crd)
		pod := builder.WithOrdinal(5).Build()

		vols := pod.Spec.Volumes
		require.Len(t, vols, 3)

		require.Equal(t, "vol-chain-home", vols[0].Name)
		require.Equal(t, "pvc-osmosis-mainnet-fullnode-5", vols[0].PersistentVolumeClaim.ClaimName)

		require.Equal(t, "vol-tmp", vols[1].Name)
		require.NotNil(t, vols[1].EmptyDir)

		require.Equal(t, "vol-config", vols[2].Name)
		require.Equal(t, "osmosis-mainnet-fullnode-5", vols[2].ConfigMap.Name)
		wantItems := []corev1.KeyToPath{
			{Key: "config-overlay.toml", Path: "config-overlay.toml"},
			{Key: "app-overlay.toml", Path: "app-overlay.toml"},
		}
		require.Equal(t, wantItems, vols[2].ConfigMap.Items)

		for _, c := range pod.Spec.Containers {
			require.Len(t, c.VolumeMounts, 1)
			mount := c.VolumeMounts[0]
			require.Equal(t, "vol-chain-home", mount.Name, c.Name)
			require.Equal(t, "/home/operator/cosmos", mount.MountPath, c.Name)
		}

		for _, c := range pod.Spec.InitContainers {
			require.Len(t, c.VolumeMounts, 3)
			mount := c.VolumeMounts[0]
			require.Equal(t, "vol-chain-home", mount.Name, c.Name)
			require.Equal(t, "/home/operator/cosmos", mount.MountPath, c.Name)

			mount = c.VolumeMounts[1]
			require.Equal(t, "vol-tmp", mount.Name, c.Name)
			require.Equal(t, "/home/operator/.tmp", mount.MountPath, c.Name)

			mount = c.VolumeMounts[2]
			require.Equal(t, "vol-config", mount.Name, c.Name)
			require.Equal(t, "/home/operator/.config", mount.MountPath, c.Name)
		}
	})

	t.Run("start container command", func(t *testing.T) {
		cmdCrd := defaultCRD()
		cmdCrd.Spec.ChainConfig.Binary = "gaiad"
		cmdCrd.Spec.PodTemplate.Image = "ghcr.io/cosmoshub:v1.2.3"

		pod := NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		c := pod.Spec.Containers[0]

		require.Equal(t, "ghcr.io/cosmoshub:v1.2.3", c.Image)

		require.Equal(t, []string{"gaiad"}, c.Command)
		require.Equal(t, []string{"start", "--home", "/home/operator/cosmos"}, c.Args)

		cmdCrd.Spec.ChainConfig.SkipInvariants = true
		pod = NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		c = pod.Spec.Containers[0]

		require.Equal(t, []string{"gaiad"}, c.Command)
		require.Equal(t, []string{"start", "--home", "/home/operator/cosmos", "--x-crisis-skip-assert-invariants"}, c.Args)

		cmdCrd.Spec.ChainConfig.LogLevel = ptr("debug")
		cmdCrd.Spec.ChainConfig.LogFormat = ptr("json")
		pod = NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		c = pod.Spec.Containers[0]

		require.Equal(t, []string{"start", "--home", "/home/operator/cosmos", "--x-crisis-skip-assert-invariants", "--log_level", "debug", "--log_format", "json"}, c.Args)
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

		require.NotEmpty(t, pod1.Labels[kube.RevisionLabel], image)
		require.NotEmpty(t, pod2.Labels[kube.RevisionLabel], image)

		require.Equal(t, pod1.Labels[kube.RevisionLabel], pod2.Labels[kube.RevisionLabel], image)

		crd.Spec.PodTemplate.Resources = corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceName(resourceName): resource.MustParse("2")}, // Changed value here.
		}
		pod3 := NewPodBuilder(&crd).Build()

		require.NotEqual(t, pod1.Labels[kube.RevisionLabel], pod3.Labels[kube.RevisionLabel])

		crd.Spec.ChainConfig.ChainID = "mychain-1"
		crd.Spec.ChainConfig.Network = "newnetwork"
		pod4 := NewPodBuilder(&crd).Build()

		require.NotEqual(t, pod3.Labels[kube.RevisionLabel], pod4.Labels[kube.RevisionLabel])
	})
}
