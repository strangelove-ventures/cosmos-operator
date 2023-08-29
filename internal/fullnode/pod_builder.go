package fullnode

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/healthcheck"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/strangelove-ventures/cosmos-operator/internal/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

const healthCheckPort = healthcheck.Port

// PodBuilder builds corev1.Pods
type PodBuilder struct {
	crd *cosmosv1.CosmosFullNode
	pod *corev1.Pod
}

// NewPodBuilder returns a valid PodBuilder.
//
// Panics if any argument is nil.
func NewPodBuilder(crd *cosmosv1.CosmosFullNode) PodBuilder {
	if crd == nil {
		panic(errors.New("nil CosmosFullNode"))
	}

	var (
		tpl                 = crd.Spec.PodTemplate
		startCmd, startArgs = startCmdAndArgs(crd)
		probes              = podReadinessProbes(crd)
	)

	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   crd.Namespace,
			Labels:      defaultLabels(crd),
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:           ptr(int64(1025)),
				RunAsGroup:          ptr(int64(1025)),
				RunAsNonRoot:        ptr(true),
				FSGroup:             ptr(int64(1025)),
				FSGroupChangePolicy: ptr(corev1.FSGroupChangeOnRootMismatch),
				SeccompProfile:      &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{
				// Main start container.
				{
					Name:  "node",
					Image: tpl.Image,
					// The following is a useful hack if you need to inspect the PV.
					//Command: []string{"/bin/sh"},
					//Args:    []string{"-c", `trap : TERM INT; sleep infinity & wait`},
					Command:         []string{startCmd},
					Args:            startArgs,
					Env:             envVars(crd),
					Ports:           buildPorts(crd.Spec.Type),
					Resources:       tpl.Resources,
					ReadinessProbe:  probes[0],
					ImagePullPolicy: tpl.ImagePullPolicy,
					WorkingDir:      workDir,
				},
			},
		},
	}

	// Add healtcheck sidecar
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name: "healthcheck",
		// Available images: https://github.com/orgs/strangelove-ventures/packages?repo_name=cosmos-operator
		// IMPORTANT: Must use v0.6.2 or later.
		Image:   "ghcr.io/strangelove-ventures/cosmos-operator:" + version.DockerTag(),
		Command: []string{"/manager", "healthcheck"},
		Ports:   []corev1.ContainerPort{{ContainerPort: healthCheckPort, Protocol: corev1.ProtocolTCP}},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5m"),
				corev1.ResourceMemory: resource.MustParse("16Mi"),
			},
		},
		ReadinessProbe:  probes[1],
		ImagePullPolicy: tpl.ImagePullPolicy,
	})

	preserveMergeInto(pod.Labels, tpl.Metadata.Labels)
	preserveMergeInto(pod.Annotations, tpl.Metadata.Annotations)

	return PodBuilder{
		crd: crd,
		pod: &pod,
	}
}

func podReadinessProbes(crd *cosmosv1.CosmosFullNode) []*corev1.Probe {
	if crd.Spec.PodTemplate.Probes.Strategy == cosmosv1.FullNodeProbeStrategyNone {
		return []*corev1.Probe{nil, nil}
	}

	mainProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health",
				Port:   intstr.FromInt(rpcPort),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 1,
		TimeoutSeconds:      10,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    5,
	}

	sidecarProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/",
				Port:   intstr.FromInt(healthCheckPort),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 1,
		TimeoutSeconds:      10,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	return []*corev1.Probe{mainProbe, sidecarProbe}
}

// Build assigns the CosmosFullNode crd as the owner and returns a fully constructed pod.
func (b PodBuilder) Build() (*corev1.Pod, error) {
	pod := b.pod.DeepCopy()
	if err := kube.ApplyStrategicMergePatch(pod, podPatch(b.crd)); err != nil {
		return nil, err
	}
	kube.NormalizeMetadata(&pod.ObjectMeta)
	return pod, nil
}

const (
	volChainHome = "vol-chain-home" // Stores live chain data and config files.
	volTmp       = "vol-tmp"        // Stores temporary config files for manipulation later.
	volConfig    = "vol-config"     // Items from ConfigMap.
	volSystemTmp = "vol-system-tmp" // Necessary for statesync or else you may see the error: ERR State sync failed err="failed to create chunk queue: unable to create temp dir for state sync chunks: stat /tmp: no such file or directory" module=statesync
	volNodeKey   = "vol-node-key"   // Secret containing the node key.
)

// WithOrdinal updates adds name and other metadata to the pod using "ordinal" which is the pod's
// ordered sequence. Pods have deterministic, consistent names similar to a StatefulSet instead of generated names.
func (b PodBuilder) WithOrdinal(ordinal int32) PodBuilder {
	pod := b.pod.DeepCopy()
	name := instanceName(b.crd, ordinal)

	pod.Labels[kube.InstanceLabel] = name

	pod.Name = name
	pod.Spec.InitContainers = initContainers(b.crd, name)

	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: volChainHome,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName(b.crd, ordinal)},
			},
		},
		{
			Name: volTmp,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: volConfig,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: instanceName(b.crd, ordinal)},
					Items: []corev1.KeyToPath{
						{Key: configOverlayFile, Path: configOverlayFile},
						{Key: appOverlayFile, Path: appOverlayFile},
					},
				},
			},
		},
		{
			Name: volSystemTmp,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: volNodeKey,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: nodeKeySecretName(b.crd, ordinal),
					Items: []corev1.KeyToPath{
						{Key: nodeKeyFile, Path: nodeKeyFile},
					},
				},
			},
		},
	}

	// Mounts required by all containers.
	mounts := []corev1.VolumeMount{
		{Name: volChainHome, MountPath: ChainHomeDir(b.crd)},
		{Name: volSystemTmp, MountPath: systemTmpDir},
	}
	// Additional mounts only needed for init containers.
	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].VolumeMounts = append(mounts, []corev1.VolumeMount{
			{Name: volTmp, MountPath: tmpDir},
			{Name: volConfig, MountPath: tmpConfigDir},
		}...)
	}

	// At this point, guaranteed to have at least 2 containers.
	pod.Spec.Containers[0].VolumeMounts = append(mounts, corev1.VolumeMount{
		Name: volNodeKey, MountPath: path.Join(ChainHomeDir(b.crd), "config", nodeKeyFile), SubPath: nodeKeyFile,
	})
	pod.Spec.Containers[1].VolumeMounts = []corev1.VolumeMount{
		// The healthcheck sidecar needs access to the home directory so it can read disk usage.
		{Name: volChainHome, MountPath: ChainHomeDir(b.crd), ReadOnly: true},
	}

	b.pod = pod
	return b
}

const (
	workDir        = "/home/operator"
	tmpDir         = workDir + "/.tmp"
	tmpConfigDir   = workDir + "/.config"
	infraToolImage = "ghcr.io/strangelove-ventures/infra-toolkit:v0.0.1"

	// Necessary for statesync
	systemTmpDir = "/tmp"
)

// ChainHomeDir is the abs filepath for the chain's home directory.
func ChainHomeDir(crd *cosmosv1.CosmosFullNode) string {
	if home := crd.Spec.ChainSpec.HomeDir; home != "" {
		return path.Join(workDir, home)
	}
	return workDir + "/cosmos"
}

func envVars(crd *cosmosv1.CosmosFullNode) []corev1.EnvVar {
	home := ChainHomeDir(crd)
	return []corev1.EnvVar{
		{Name: "HOME", Value: workDir},
		{Name: "CHAIN_HOME", Value: home},
		{Name: "GENESIS_FILE", Value: path.Join(home, "config", "genesis.json")},
		{Name: "CONFIG_DIR", Value: path.Join(home, "config")},
		{Name: "DATA_DIR", Value: path.Join(home, "data")},
	}
}

func initContainers(crd *cosmosv1.CosmosFullNode, moniker string) []corev1.Container {
	tpl := crd.Spec.PodTemplate
	binary := crd.Spec.ChainSpec.Binary
	genesisCmd, genesisArgs := DownloadGenesisCommand(crd.Spec.ChainSpec)
	env := envVars(crd)

	initCmd := fmt.Sprintf("%s init %s --chain-id %s", binary, moniker, crd.Spec.ChainSpec.ChainID)
	required := []corev1.Container{
		{
			Name:            "clean-init",
			Image:           infraToolImage,
			Command:         []string{"sh"},
			Args:            []string{"-c", `rm -rf "$HOME/.tmp/*"`},
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		},
		{
			Name:    "chain-init",
			Image:   tpl.Image,
			Command: []string{"sh"},
			Args: []string{"-c",
				fmt.Sprintf(`
set -eu
if [ ! -d "$CHAIN_HOME/data" ]; then
	echo "Initializing chain..."
	%s --home "$CHAIN_HOME"
else
	echo "Skipping chain init; already initialized."
fi

echo "Initializing into tmp dir for downstream processing..."
%s --home "$HOME/.tmp"
`, initCmd, initCmd),
			},
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		},

		{
			Name:            "genesis-init",
			Image:           infraToolImage,
			Command:         []string{genesisCmd},
			Args:            genesisArgs,
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		},

		{
			Name:    "config-merge",
			Image:   infraToolImage,
			Command: []string{"sh"},
			Args: []string{"-c",
				`
set -eu
CONFIG_DIR="$CHAIN_HOME/config"
TMP_DIR="$HOME/.tmp/config"
OVERLAY_DIR="$HOME/.config"

# This is a hack to prevent adding another init container.
# Ideally, this step is not concerned with merging config, so it would live elsewhere.
# The node key is a secret mounted into the main "node" container, so we do not need this one.
echo "Removing node key from chain's init subcommand..."
rm -rf "$CONFIG_DIR/node_key.json"

echo "Merging config..."
set -x
config-merge -f toml "$TMP_DIR/config.toml" "$OVERLAY_DIR/config-overlay.toml" > "$CONFIG_DIR/config.toml"
config-merge -f toml "$TMP_DIR/app.toml" "$OVERLAY_DIR/app-overlay.toml" > "$CONFIG_DIR/app.toml"
`,
			},
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		},
	}

	if willRestoreFromSnapshot(crd) {
		cmd, args := DownloadSnapshotCommand(crd.Spec.ChainSpec)
		required = append(required, corev1.Container{
			Name:            "snapshot-restore",
			Image:           infraToolImage,
			Command:         []string{cmd},
			Args:            args,
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		})
	}

	return required
}

func startCmdAndArgs(crd *cosmosv1.CosmosFullNode) (string, []string) {
	var (
		binary             = crd.Spec.ChainSpec.Binary
		args               = startCommandArgs(crd)
		privvalSleep int32 = 10
	)
	if v := crd.Spec.ChainSpec.PrivvalSleepSeconds; v != nil {
		privvalSleep = *v
	}

	if crd.Spec.Type == cosmosv1.Sentry && privvalSleep > 0 {
		shellBody := fmt.Sprintf(`sleep %d
%s %s`, privvalSleep, binary, strings.Join(args, " "))
		return "sh", []string{"-c", shellBody}
	}

	return binary, args
}

func startCommandArgs(crd *cosmosv1.CosmosFullNode) []string {
	args := []string{"start", "--home", ChainHomeDir(crd)}
	cfg := crd.Spec.ChainSpec
	if cfg.SkipInvariants {
		args = append(args, "--x-crisis-skip-assert-invariants")
	}
	if lvl := cfg.LogLevel; lvl != nil {
		args = append(args, "--log_level", *lvl)
	}
	if format := cfg.LogFormat; format != nil {
		args = append(args, "--log_format", *format)
	}
	return args
}

func willRestoreFromSnapshot(crd *cosmosv1.CosmosFullNode) bool {
	return crd.Spec.ChainSpec.SnapshotURL != nil || crd.Spec.ChainSpec.SnapshotScript != nil
}

func podPatch(crd *cosmosv1.CosmosFullNode) *corev1.Pod {
	tpl := crd.Spec.PodTemplate
	// For fields with sliceOrDefault if you pass nil, the field is deleted.
	spec := corev1.PodSpec{
		Affinity:                      tpl.Affinity,
		Containers:                    sliceOrDefault(tpl.Containers, []corev1.Container{}),
		ImagePullSecrets:              sliceOrDefault(tpl.ImagePullSecrets, []corev1.LocalObjectReference{}),
		InitContainers:                sliceOrDefault(tpl.InitContainers, []corev1.Container{}),
		NodeSelector:                  tpl.NodeSelector,
		Priority:                      tpl.Priority,
		PriorityClassName:             tpl.PriorityClassName,
		TerminationGracePeriodSeconds: valOrDefault(tpl.TerminationGracePeriodSeconds, ptr(int64(30))),
		Tolerations:                   sliceOrDefault(tpl.Tolerations, []corev1.Toleration{}),
		Volumes:                       sliceOrDefault(tpl.Volumes, []corev1.Volume{}),
	}
	return &corev1.Pod{Spec: spec}
}

// PVCName returns the primary PVC holding the chain data associated with the pod.
func PVCName(pod *corev1.Pod) string {
	found, ok := lo.Find(pod.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == volChainHome })
	if !ok {
		return ""
	}
	if found.PersistentVolumeClaim == nil {
		return ""
	}
	return found.PersistentVolumeClaim.ClaimName
}
