package fullnode

import (
	"strings"
	"testing"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
	"github.com/bharvest-devops/cosmos-operator/internal/test"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func defaultCRD() cosmosv1.CosmosFullNode {
	cometConfig := cosmosv1.CometConfig{}
	appConfig := cosmosv1.SDKAppConfig{}
	return cosmosv1.CosmosFullNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "osmosis",
			Namespace:       "test",
			ResourceVersion: "_resource_version_",
		},
		Spec: cosmosv1.FullNodeSpec{
			ChainSpec: cosmosv1.ChainSpec{
				Network:   "mainnet",
				CosmosSDK: &appConfig,
				Comet:     &cometConfig,
			},
			PodTemplate: cosmosv1.PodSpec{
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
			VolumeClaimTemplate: cosmosv1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("100Gi")},
				},
			},
		},
	}
}

func TestPodBuilder(t *testing.T) {
	t.Parallel()

	t.Run("happy path - critical fields", func(t *testing.T) {
		crd := defaultCRD()
		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(5).Build()
		require.NoError(t, err)

		require.Equal(t, "Pod", pod.Kind)
		require.Equal(t, "v1", pod.APIVersion)

		require.Equal(t, "test", pod.Namespace)
		require.Equal(t, "osmosis-5", pod.Name)

		require.Equal(t, "osmosis", pod.Spec.Subdomain)
		require.Equal(t, "osmosis-5", pod.Spec.Hostname)

		wantLabels := map[string]string{
			"app.kubernetes.io/instance":   "osmosis-5",
			"app.kubernetes.io/component":  "CosmosFullNode",
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/name":       "osmosis",
			"app.kubernetes.io/version":    "v1.2.3",
			"cosmos.bharvest/network":      "mainnet",
			"cosmos.bharvest/type":         "FullNode",
		}
		require.Equal(t, wantLabels, pod.Labels)
		require.NotNil(t, pod.Annotations)
		require.Empty(t, pod.Annotations)

		require.EqualValues(t, 30, *pod.Spec.TerminationGracePeriodSeconds)

		sc := pod.Spec.SecurityContext
		require.EqualValues(t, 1025, *sc.RunAsUser)
		require.EqualValues(t, 1025, *sc.RunAsGroup)
		require.EqualValues(t, 1025, *sc.FSGroup)
		require.EqualValues(t, "OnRootMismatch", *sc.FSGroupChangePolicy)
		require.True(t, *sc.RunAsNonRoot)
		require.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, sc.SeccompProfile.Type)

		// Test we don't share or leak data per invocation.
		pod, err = builder.Build()
		require.NoError(t, err)
		require.Empty(t, pod.Name)

		pod, err = builder.WithOrdinal(123).Build()
		require.NoError(t, err)
		require.Equal(t, "osmosis-123", pod.Name)

		crd.Spec.Type = cosmosv1.FullNode
		pod2, err := NewPodBuilder(&crd).WithOrdinal(123).Build()
		require.NoError(t, err)
		require.Equal(t, pod, pod2)
	})

	t.Run("happy path - ports", func(t *testing.T) {
		crd := defaultCRD()
		pod, err := NewPodBuilder(&crd).Build()
		require.NoError(t, err)
		ports := pod.Spec.Containers[0].Ports

		require.Equal(t, 7, len(ports))

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
			{"grpc-web", 9091},
		} {
			port := ports[i]
			require.Equal(t, tt.Name, port.Name, tt)
			require.Equal(t, corev1.ProtocolTCP, port.Protocol)
			require.Equal(t, tt.Port, port.ContainerPort)
			require.Zero(t, port.HostPort)
		}
	})

	t.Run("ports - sentry", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Type = cosmosv1.Sentry

		pod, err := NewPodBuilder(&crd).Build()
		require.NoError(t, err)
		ports := pod.Spec.Containers[0].Ports

		require.Equal(t, 8, len(ports))

		got, _ := lo.Last(ports)

		require.Equal(t, "privval", got.Name)
		require.Equal(t, corev1.ProtocolTCP, got.Protocol)
		require.EqualValues(t, 1234, got.ContainerPort)
		require.Zero(t, got.HostPort)
	})

	t.Run("happy path - optional fields", func(t *testing.T) {
		optCrd := defaultCRD()

		optCrd.Spec.PodTemplate.Metadata.Labels = map[string]string{"custom": "label", kube.NameLabel: "should not see me"}
		optCrd.Spec.PodTemplate.Metadata.Annotations = map[string]string{"custom": "annotation"}

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
		pod, err := builder.WithOrdinal(9).Build()
		require.NoError(t, err)

		require.Equal(t, "label", pod.Labels["custom"])
		// Operator label takes precedence.
		require.Equal(t, "osmosis", pod.Labels[kube.NameLabel])

		require.Equal(t, "annotation", pod.Annotations["custom"])

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
		pod, err := builder.WithOrdinal(125).Build()
		require.NoError(t, err)

		require.Regexp(t, `a.*-125`, pod.Name)

		test.RequireValidMetadata(t, pod)
	})

	t.Run("additional args", func(t *testing.T) {
		crd := defaultCRD()

		crd.Spec.ChainSpec.AdditionalStartArgs = []string{"--foo", "bar"}

		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(0).Build()
		require.NoError(t, err)

		test.RequireValidMetadata(t, pod)

		require.Equal(t, []string{"start", "--home", "/home/operator/cosmos", "--foo", "bar"}, pod.Spec.Containers[0].Args)
	})

	t.Run("containers", func(t *testing.T) {
		crd := defaultCRD()
		const wantWrkDir = "/home/operator"
		crd.Spec.ChainSpec.ChainID = "osmosis-123"
		crd.Spec.ChainSpec.ChainType = chainTypeCosmos
		crd.Spec.ChainSpec.Binary = "osmosisd"
		crd.Spec.ChainSpec.CosmosSDK.SnapshotURL = ptr("https://example.com/snapshot.tar")
		crd.Spec.PodTemplate.Image = "main-image:v1.2.3"
		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(6).Build()
		require.NoError(t, err)

		require.Len(t, pod.Spec.Containers, 2)

		startContainer := pod.Spec.Containers[0]
		require.Equal(t, "node", startContainer.Name)
		require.Empty(t, startContainer.ImagePullPolicy)
		require.Equal(t, crd.Spec.PodTemplate.Resources, startContainer.Resources)
		require.Equal(t, wantWrkDir, startContainer.WorkingDir)

		require.Equal(t, startContainer.Env[0].Name, "HOME")
		require.Equal(t, startContainer.Env[0].Value, "/home/operator")
		require.Equal(t, startContainer.Env[1].Name, "CHAIN_HOME")
		require.Equal(t, startContainer.Env[1].Value, "/home/operator/cosmos")
		require.Equal(t, startContainer.Env[2].Name, "GENESIS_FILE")
		require.Equal(t, startContainer.Env[2].Value, "/home/operator/cosmos/config/genesis.json")
		require.Equal(t, startContainer.Env[3].Name, "ADDRBOOK_FILE")
		require.Equal(t, startContainer.Env[3].Value, "/home/operator/cosmos/config/addrbook.json")
		require.Equal(t, startContainer.Env[4].Name, "CONFIG_DIR")
		require.Equal(t, startContainer.Env[4].Value, "/home/operator/cosmos/config")
		require.Equal(t, startContainer.Env[5].Name, "DATA_DIR")
		require.Equal(t, startContainer.Env[5].Value, "/home/operator/cosmos/data")
		require.Equal(t, startContainer.Env[6].Name, "CHAIN_ID")
		require.Equal(t, startContainer.Env[6].Value, crd.Spec.ChainSpec.ChainID)
		require.Equal(t, startContainer.Env[7].Name, "CHAIN_TYPE")
		require.Equal(t, startContainer.Env[7].Value, crd.Spec.ChainSpec.ChainType)
		require.Equal(t, envVars(&crd), startContainer.Env)

		healthContainer := pod.Spec.Containers[1]
		require.Equal(t, "healthcheck", healthContainer.Name)
		//require.Equal(t, "ghcr.io/strangelove-ventures/cosmos-operator:latest", healthContainer.Image)
		require.Equal(t, "ghcr.io/bharvest-devops/cosmos-operator:latest", healthContainer.Image)
		require.Equal(t, []string{"/manager", "healthcheck"}, healthContainer.Command)
		require.Empty(t, healthContainer.Args)
		require.Empty(t, healthContainer.ImagePullPolicy)
		require.NotEmpty(t, healthContainer.Resources)
		require.Empty(t, healthContainer.Env)
		healthPort := corev1.ContainerPort{
			ContainerPort: 1251,
			Protocol:      "TCP",
		}
		require.Equal(t, healthPort, healthContainer.Ports[0])

		require.Len(t, lo.Map(pod.Spec.InitContainers, func(c corev1.Container, _ int) string { return c.Name }), 7)

		wantInitImages := []string{
			"ghcr.io/bharvest-devops/infratoolkit:v0.1.0",
			"main-image:v1.2.3",
			"ghcr.io/bharvest-devops/infratoolkit:v0.1.0",
			"ghcr.io/bharvest-devops/infratoolkit:v0.1.0",
			"ghcr.io/bharvest-devops/infratoolkit:v0.1.0",
			"ghcr.io/bharvest-devops/infratoolkit:v0.1.0",
			//"ghcr.io/strangelove-ventures/cosmos-operator:latest",
			"ghcr.io/bharvest-devops/cosmos-operator:latest",
		}
		require.Equal(t, wantInitImages, lo.Map(pod.Spec.InitContainers, func(c corev1.Container, _ int) string {
			return c.Image
		}))

		for _, c := range pod.Spec.InitContainers {
			require.Equal(t, envVars(&crd), startContainer.Env, c.Name)
			require.Equal(t, wantWrkDir, c.WorkingDir)
		}

		freshCont := pod.Spec.InitContainers[0]
		require.Contains(t, freshCont.Args[1], `rm -rf "$HOME/.tmp/*"`)

		initCont := pod.Spec.InitContainers[1]
		require.Contains(t, initCont.Args[1], `osmosisd init --chain-id osmosis-123 osmosis-6 --home "$CHAIN_HOME"`)
		require.Contains(t, initCont.Args[1], `osmosisd init --chain-id osmosis-123 osmosis-6 --home "$HOME/.tmp"`)

		mergeConfig1 := pod.Spec.InitContainers[3]
		// The order of config-merge arguments is important. Rightmost takes precedence.
		require.Contains(t, mergeConfig1.Args[1], `echo Using default address book`)

		mergeConfig := pod.Spec.InitContainers[4]
		// The order of config-merge arguments is important. Rightmost takes precedence.
		require.Contains(t, mergeConfig.Args[1], `config-merge -f toml "$TMP_DIR/config.toml" "$OVERLAY_DIR/config-overlay.toml" > "$CONFIG_DIR/config.toml"`)
		require.Contains(t, mergeConfig.Args[1], `config-merge -f toml "$TMP_DIR/app.toml" "$OVERLAY_DIR/app-overlay.toml" > "$CONFIG_DIR/app.toml`)
	})

	t.Run("containers - configured home dir", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.ChainSpec.HomeDir = ".osmosisd"
		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(6).Build()
		require.NoError(t, err)

		require.Len(t, pod.Spec.Containers, 2)

		container := pod.Spec.Containers[0]
		require.Equal(t, "node", container.Name)
		require.Empty(t, container.ImagePullPolicy)
		require.Equal(t, crd.Spec.PodTemplate.Resources, container.Resources)

		require.Equal(t, container.Env[0].Name, "HOME")
		require.Equal(t, container.Env[0].Value, "/home/operator")
		require.Equal(t, container.Env[1].Name, "CHAIN_HOME")
		require.Equal(t, container.Env[1].Value, "/home/operator/.osmosisd")
		require.Equal(t, container.Env[2].Name, "GENESIS_FILE")
		require.Equal(t, container.Env[2].Value, "/home/operator/.osmosisd/config/genesis.json")
		require.Equal(t, container.Env[3].Name, "ADDRBOOK_FILE")
		require.Equal(t, container.Env[3].Value, "/home/operator/.osmosisd/config/addrbook.json")
		require.Equal(t, container.Env[4].Name, "CONFIG_DIR")
		require.Equal(t, container.Env[4].Value, "/home/operator/.osmosisd/config")
		require.Equal(t, container.Env[5].Name, "DATA_DIR")
		require.Equal(t, container.Env[5].Value, "/home/operator/.osmosisd/data")
		require.Equal(t, container.Env[6].Name, "CHAIN_ID")
		require.Equal(t, container.Env[6].Value, crd.Spec.ChainSpec.ChainID)
		require.Equal(t, container.Env[7].Name, "CHAIN_TYPE")
		require.Equal(t, container.Env[7].Value, crd.Spec.ChainSpec.ChainType)

		require.NotEmpty(t, pod.Spec.InitContainers)

		for _, c := range pod.Spec.InitContainers {
			require.Equal(t, container.Env, c.Env, c.Name)
		}
	})

	t.Run("volumes", func(t *testing.T) {
		crd := defaultCRD()
		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(5).Build()
		require.NoError(t, err)

		vols := pod.Spec.Volumes
		require.Equal(t, 5, len(vols))

		require.Equal(t, "vol-chain-home", vols[0].Name)
		require.Equal(t, "pvc-osmosis-5", vols[0].PersistentVolumeClaim.ClaimName)

		require.Equal(t, "vol-tmp", vols[1].Name)
		require.NotNil(t, vols[1].EmptyDir)

		require.Equal(t, "vol-config", vols[2].Name)
		require.Equal(t, "osmosis-5", vols[2].ConfigMap.Name)
		wantItems := []corev1.KeyToPath{
			{Key: "config-overlay.toml", Path: "config-overlay.toml"},
			{Key: "app-overlay.toml", Path: "app-overlay.toml"},
		}
		require.Equal(t, wantItems, vols[2].ConfigMap.Items)

		// Required for statesync
		require.Equal(t, "vol-system-tmp", vols[3].Name)
		require.NotNil(t, vols[3].EmptyDir)

		// Node key
		require.Equal(t, "vol-node-key", vols[4].Name)
		require.Equal(t, "osmosis-node-key-5", vols[4].Secret.SecretName)
		require.Equal(t, []corev1.KeyToPath{{Key: "node_key.json", Path: "node_key.json"}}, vols[4].Secret.Items)

		require.Equal(t, len(pod.Spec.Containers), 2)

		c := pod.Spec.Containers[0]
		require.Equal(t, "node", c.Name) // Sanity check

		require.Len(t, c.VolumeMounts, 3)
		mount := c.VolumeMounts[0]
		require.Equal(t, "vol-chain-home", mount.Name)
		require.Equal(t, "/home/operator/cosmos", mount.MountPath)
		require.False(t, mount.ReadOnly)

		mount = c.VolumeMounts[1]
		require.Equal(t, "vol-system-tmp", mount.Name)
		require.Equal(t, "/tmp", mount.MountPath)
		require.False(t, mount.ReadOnly)

		mount = c.VolumeMounts[2]
		require.Equal(t, "vol-node-key", mount.Name)
		require.Equal(t, "/home/operator/cosmos/config/node_key.json", mount.MountPath)
		require.Equal(t, "node_key.json", mount.SubPath)

		// healtcheck sidecar
		c = pod.Spec.Containers[1]
		require.Equal(t, 1, len(c.VolumeMounts))
		require.Equal(t, "healthcheck", c.Name) // Sanity check
		mount = c.VolumeMounts[0]
		require.Equal(t, "vol-chain-home", mount.Name)
		require.Equal(t, "/home/operator/cosmos", mount.MountPath)
		require.True(t, mount.ReadOnly)

		for _, c := range pod.Spec.InitContainers {
			require.Len(t, c.VolumeMounts, 4)
			mount := c.VolumeMounts[0]
			require.Equal(t, "vol-chain-home", mount.Name, c.Name)
			require.Equal(t, "/home/operator/cosmos", mount.MountPath, c.Name)

			mount = c.VolumeMounts[1]
			require.Equal(t, "vol-system-tmp", mount.Name, c.Name)
			require.Equal(t, "/tmp", mount.MountPath, c.Name)

			mount = c.VolumeMounts[2]
			require.Equal(t, "vol-tmp", mount.Name, c.Name)
			require.Equal(t, "/home/operator/.tmp", mount.MountPath, c.Name)

			mount = c.VolumeMounts[3]
			require.Equal(t, "vol-config", mount.Name, c.Name)
			require.Equal(t, "/home/operator/.config", mount.MountPath, c.Name)
		}
	})

	t.Run("start container command", func(t *testing.T) {
		const defaultHome = "/home/operator/cosmos"

		cmdCrd := defaultCRD()
		cmdCrd.Spec.ChainSpec.Binary = "gaiad"
		cmdCrd.Spec.PodTemplate.Image = "ghcr.io/cosmoshub:v1.2.3"

		pod, err := NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		require.NoError(t, err)
		c := pod.Spec.Containers[0]

		require.Equal(t, "ghcr.io/cosmoshub:v1.2.3", c.Image)

		require.Equal(t, []string{"gaiad"}, c.Command)
		require.Equal(t, []string{"start", "--home", defaultHome}, c.Args)

		cmdCrd.Spec.ChainSpec.CosmosSDK.SkipInvariants = true
		pod, err = NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		require.NoError(t, err)
		c = pod.Spec.Containers[0]

		require.Equal(t, []string{"gaiad"}, c.Command)
		require.Equal(t, []string{"start", "--home", defaultHome, "--x-crisis-skip-assert-invariants"}, c.Args)

		cmdCrd.Spec.ChainSpec.LogLevel = ptr("debug")
		cmdCrd.Spec.ChainSpec.LogFormat = ptr("json")
		pod, err = NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		require.NoError(t, err)
		c = pod.Spec.Containers[0]

		require.Equal(t, []string{"start", "--home", defaultHome, "--x-crisis-skip-assert-invariants", "--log_level", "debug", "--log_format", "json"}, c.Args)

		cmdCrd.Spec.ChainSpec.HomeDir = ".other"
		pod, err = NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		require.NoError(t, err)

		c = pod.Spec.Containers[0]
		require.Equal(t, []string{"start", "--home", "/home/operator/.other", "--x-crisis-skip-assert-invariants", "--log_level", "debug", "--log_format", "json"}, c.Args)
	})

	t.Run("sentry start container command ", func(t *testing.T) {
		cmdCrd := defaultCRD()
		cmdCrd.Spec.ChainSpec.Binary = "gaiad"
		cmdCrd.Spec.Type = cosmosv1.Sentry

		pod, err := NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		require.NoError(t, err)
		c := pod.Spec.Containers[0]

		require.Equal(t, []string{"sh"}, c.Command)
		const wantBody1 = `sleep 10
gaiad start --home /home/operator/cosmos`
		require.Equal(t, []string{"-c", wantBody1}, c.Args)

		cmdCrd.Spec.ChainSpec.PrivvalSleepSeconds = ptr(int32(60))
		pod, err = NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		require.NoError(t, err)
		c = pod.Spec.Containers[0]

		const wantBody2 = `sleep 60
gaiad start --home /home/operator/cosmos`
		require.Equal(t, []string{"-c", wantBody2}, c.Args)

		cmdCrd.Spec.ChainSpec.PrivvalSleepSeconds = ptr(int32(0))
		pod, err = NewPodBuilder(&cmdCrd).WithOrdinal(1).Build()
		require.NoError(t, err)
		c = pod.Spec.Containers[0]

		require.Equal(t, []string{"gaiad"}, c.Command)
	})

	t.Run("rpc probes", func(t *testing.T) {
		crd := defaultCRD()
		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(1).Build()
		require.NoError(t, err)

		want := &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/health",
					Port:   intstr.FromInt(26657),
					Scheme: "HTTP",
				},
			},
			InitialDelaySeconds: 1,
			TimeoutSeconds:      10,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    5,
		}
		got := pod.Spec.Containers[0].ReadinessProbe

		require.Equal(t, want, got)

		want = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/",
					Port:   intstr.FromInt(1251),
					Scheme: "HTTP",
				},
			},
			InitialDelaySeconds: 1,
			TimeoutSeconds:      10,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}
		got = pod.Spec.Containers[1].ReadinessProbe

		require.Equal(t, want, got)
	})

	t.Run("probe strategy", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.PodTemplate.Probes = cosmosv1.FullNodeProbesSpec{Strategy: cosmosv1.FullNodeProbeStrategyNone}

		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(1).Build()
		require.NoError(t, err)

		for i, cont := range pod.Spec.Containers {
			require.Nilf(t, cont.ReadinessProbe, "container %d", i)
		}

		require.Equal(t, 2, len(pod.Spec.Containers))
		require.Equal(t, "node", pod.Spec.Containers[0].Name)

		sidecar := pod.Spec.Containers[1]
		require.Equal(t, "healthcheck", sidecar.Name)
		require.Nil(t, sidecar.ReadinessProbe)
	})

	t.Run("strategic merge fields", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.PodTemplate.Volumes = []corev1.Volume{
			{Name: "foo-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		}
		crd.Spec.PodTemplate.InitContainers = []corev1.Container{
			{Name: "chain-init", Image: "foo:latest", VolumeMounts: []corev1.VolumeMount{
				{Name: "foo-vol", MountPath: "/foo"}, // Should be merged with existing.
			}},
			{Name: "new-init", Image: "new-init:latest"}, // New container.
		}
		crd.Spec.PodTemplate.Containers = []corev1.Container{
			{Name: "node", VolumeMounts: []corev1.VolumeMount{
				{Name: "foo-vol", MountPath: "/foo"}, // Should be merged with existing.
			}},
			{Name: "new-sidecar", Image: "new-sidecar:latest"}, // New container.
		}

		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(0).Build()
		require.NoError(t, err)

		vols := lo.SliceToMap(pod.Spec.Volumes, func(v corev1.Volume) (string, corev1.Volume) { return v.Name, v })
		require.ElementsMatch(t, []string{"foo-vol", "vol-tmp", "vol-system-tmp", "vol-config", "vol-chain-home", "vol-node-key"}, lo.Keys(vols))
		require.Equal(t, &corev1.EmptyDirVolumeSource{}, vols["foo-vol"].VolumeSource.EmptyDir)

		containers := lo.SliceToMap(pod.Spec.Containers, func(c corev1.Container) (string, corev1.Container) { return c.Name, c })
		require.ElementsMatch(t, []string{"node", "new-sidecar", "healthcheck"}, lo.Keys(containers))

		extraVol := lo.Filter(containers["node"].VolumeMounts, func(vm corev1.VolumeMount, _ int) bool { return vm.Name == "foo-vol" })
		require.Equal(t, "/foo", extraVol[0].MountPath)

		initConts := lo.SliceToMap(pod.Spec.InitContainers, func(c corev1.Container) (string, corev1.Container) { return c.Name, c })
		require.ElementsMatch(t, []string{"clean-init", "chain-init", "new-init", "genesis-init", "addrbook-init", "config-merge", "version-check"}, lo.Keys(initConts))
		require.Equal(t, "foo:latest", initConts["chain-init"].Image)
	})

	t.Run("containers with chain spec versions", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.PodTemplate.Volumes = []corev1.Volume{
			{Name: "foo-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		}
		crd.Spec.PodTemplate.InitContainers = []corev1.Container{
			{Name: "chain-init", Image: "foo:latest", VolumeMounts: []corev1.VolumeMount{
				{Name: "foo-vol", MountPath: "/foo"}, // Should be merged with existing.
			}},
			{Name: "new-init", Image: "new-init:latest"}, // New container.
		}
		crd.Spec.PodTemplate.Containers = []corev1.Container{
			{Name: "node", VolumeMounts: []corev1.VolumeMount{
				{Name: "foo-vol", MountPath: "/foo"}, // Should be merged with existing.
			}},
			{Name: "new-sidecar", Image: "new-sidecar:latest"}, // New container.
		}
		crd.Spec.ChainSpec.Versions = []cosmosv1.ChainVersion{
			{
				UpgradeHeight: 1,
				Image:         "image:v1.0.0",
			},
			{
				UpgradeHeight: 100,
				Image:         "image:v2.0.0",
			},
		}

		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(0).Build()
		require.NoError(t, err)

		containers := lo.SliceToMap(pod.Spec.Containers, func(c corev1.Container) (string, corev1.Container) { return c.Name, c })
		require.ElementsMatch(t, []string{"node", "new-sidecar", "healthcheck", "version-check-interval"}, lo.Keys(containers))
	})

	test.HasTypeLabel(t, func(crd cosmosv1.CosmosFullNode) []map[string]string {
		cometConfig := cosmosv1.CometConfig{}
		appConfig := cosmosv1.SDKAppConfig{}
		crd.Spec.ChainSpec.Comet = &cometConfig
		crd.Spec.ChainSpec.CosmosSDK = &appConfig
		builder := NewPodBuilder(&crd)
		pod, _ := builder.WithOrdinal(5).Build()
		return []map[string]string{pod.Labels}
	})
}

func TestChainHomeDir(t *testing.T) {
	crd := defaultCRD()
	require.Equal(t, "/home/operator/cosmos", ChainHomeDir(&crd))

	crd.Spec.ChainSpec.HomeDir = ".gaia"
	require.Equal(t, "/home/operator/.gaia", ChainHomeDir(&crd))
}

func TestPVCName(t *testing.T) {
	crd := defaultCRD()
	appConfig := cosmosv1.SDKAppConfig{}
	crd.Spec.ChainSpec.CosmosSDK = &appConfig
	builder := NewPodBuilder(&crd)
	pod, err := builder.WithOrdinal(5).Build()
	require.NoError(t, err)

	require.Equal(t, "pvc-osmosis-5", PVCName(pod))

	pod.Spec.Volumes = append([]corev1.Volume{{Name: "foo"}}, pod.Spec.Volumes...)

	require.Equal(t, "pvc-osmosis-5", PVCName(pod))
}
