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

const (
	healthCheckPort    = healthcheck.Port
	mainContainer      = "node"
	chainInitContainer = "chain-init"
)

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

	versionCheckCmd := []string{"/manager", "versioncheck", "-d"}
	if crd.Spec.ChainSpec.DatabaseBackend != nil {
		versionCheckCmd = append(versionCheckCmd, "-b", *crd.Spec.ChainSpec.DatabaseBackend)
	}

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
			ServiceAccountName: serviceAccountName(crd),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:           ptr(int64(1025)),
				RunAsGroup:          ptr(int64(1025)),
				RunAsNonRoot:        ptr(true),
				FSGroup:             ptr(int64(1025)),
				FSGroupChangePolicy: ptr(corev1.FSGroupChangeOnRootMismatch),
				SeccompProfile:      &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Subdomain: crd.Name,
			Containers: []corev1.Container{
				// Main start container.
				{
					Name:  mainContainer,
					Image: tpl.Image,
					// The following is a useful hack if you need to inspect the PV.
					//Command: []string{"/bin/sh"},
					//Args:    []string{"-c", `trap : TERM INT; sleep infinity & wait`},
					Command:         []string{startCmd},
					Args:            startArgs,
					Env:             envVars(crd),
					Ports:           buildPorts(crd),
					Resources:       tpl.Resources,
					ReadinessProbe:  probes[0],
					ImagePullPolicy: tpl.ImagePullPolicy,
					WorkingDir:      workDir,
				},
				// healthcheck sidecar
				{
					Name: "healthcheck",
					// Available images: https://github.com/orgs/strangelove-ventures/packages?repo_name=cosmos-operator
					// IMPORTANT: Must use v0.6.2 or later.
					Image:   version.Image() + ":" + version.DockerTag(),
					Command: []string{"/manager", "healthcheck", "--rpc-host", fmt.Sprintf("http://localhost:%d", crd.Spec.ChainSpec.Comet.RPCPort())},
					Ports:   []corev1.ContainerPort{{ContainerPort: healthCheckPort, Protocol: corev1.ProtocolTCP}},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("5m"),
							corev1.ResourceMemory: resource.MustParse("16Mi"),
						},
					},
					ReadinessProbe:  probes[1],
					ImagePullPolicy: tpl.ImagePullPolicy,
				},
			},
		},
	}

	if len(crd.Spec.ChainSpec.Versions) > 0 {
		// version check sidecar, runs on inverval in case the instance is halting for upgrade.
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
			Name:    "version-check-interval",
			Image:   version.Image() + ":" + version.DockerTag(),
			Command: versionCheckCmd,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("5m"),
					corev1.ResourceMemory: resource.MustParse("16Mi"),
				},
			},
			Env:             envVars(crd),
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
			SecurityContext: &corev1.SecurityContext{},
		})
	}

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
				Port:   intstr.FromInt(int(crd.Spec.ChainSpec.Comet.RPCPort())),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 1,
		TimeoutSeconds:      10,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    5,
	}

	if crd.Spec.PodTemplate.Probes.Strategy == cosmosv1.FullNodeProbeStrategyReachable {
		return []*corev1.Probe{mainProbe, nil}
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

	if len(b.crd.Spec.ChainSpec.Versions) > 0 {
		instanceHeight := uint64(0)
		if height, ok := b.crd.Status.Height[pod.Name]; ok {
			instanceHeight = height
		}
		var vrs *cosmosv1.ChainVersion
		for _, v := range b.crd.Spec.ChainSpec.Versions {
			if instanceHeight < v.UpgradeHeight {
				break
			}
			vrs = &v
		}
		if vrs != nil {
			setVersionedImages(pod, vrs)
		}
	}

	if o, ok := b.crd.Spec.InstanceOverrides[pod.Name]; ok {
		if o.DisableStrategy != nil {
			return nil, nil
		}
		if o.Image != "" {
			setChainContainerImage(pod, o.Image)
		}
		if o.NodeSelector != nil {
			pod.Spec.NodeSelector = o.NodeSelector
		}
	}

	kube.NormalizeMetadata(&pod.ObjectMeta)
	return pod, nil
}

const (
	volChainHome = "vol-chain-home" // Stores live chain data and config files.
	volTmp       = "vol-tmp"        // Stores temporary config files for manipulation later.
	volConfig    = "vol-config"     // Overlay items from ConfigMap.
	volSystemTmp = "vol-system-tmp" // Necessary for statesync or else you may see the error: ERR State sync failed err="failed to create chunk queue: unable to create temp dir for state sync chunks: stat /tmp: no such file or directory" module=statesync
)

// WithOrdinal updates adds name and other metadata to the pod using "ordinal" which is the pod's
// ordered sequence. Pods have deterministic, consistent names similar to a StatefulSet instead of generated names.
func (b PodBuilder) WithOrdinal(ordinal int32) PodBuilder {
	pod := b.pod.DeepCopy()
	name := instanceName(b.crd, ordinal)

	pod.Labels[kube.InstanceLabel] = name

	pod.Name = name
	pod.Spec.InitContainers = initContainers(b.crd, name)

	pod.Spec.Hostname = pod.Name
	pod.Spec.Subdomain = b.crd.Name

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
						{Key: nodeKeyFile, Path: nodeKeyFile},
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
	pod.Spec.Containers[0].VolumeMounts = mounts
	pod.Spec.Containers[1].VolumeMounts = []corev1.VolumeMount{
		// The healthcheck sidecar needs access to the home directory so it can read disk usage.
		{Name: volChainHome, MountPath: ChainHomeDir(b.crd), ReadOnly: true},
	}
	if len(pod.Spec.Containers) > 2 {
		pod.Spec.Containers[2].VolumeMounts = mounts
	}

	b.pod = pod
	return b
}

const (
	workDir          = "/home/operator"
	tmpDir           = workDir + "/.tmp"
	tmpConfigDir     = workDir + "/.config"
	infraToolImage   = "ghcr.io/strangelove-ventures/infra-toolkit"
	infraToolVersion = "v0.1.6"

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
		{Name: "ADDRBOOK_FILE", Value: path.Join(home, "config", "addrbook.json")},
		{Name: "CONFIG_DIR", Value: path.Join(home, "config")},
		{Name: "DATA_DIR", Value: path.Join(home, "data")},
	}
}

func resolveInfraToolImage() string {
	return fmt.Sprintf("%s:%s", infraToolImage, infraToolVersion)
}

func initContainers(crd *cosmosv1.CosmosFullNode, moniker string) []corev1.Container {
	tpl := crd.Spec.PodTemplate
	binary := crd.Spec.ChainSpec.Binary
	genesisCmd, genesisArgs := DownloadGenesisCommand(crd.Spec.ChainSpec)
	addrbookCmd, addrbookArgs := DownloadAddrbookCommand(crd.Spec.ChainSpec)
	env := envVars(crd)

	initCmd := fmt.Sprintf("%s init --chain-id %s %s", binary, crd.Spec.ChainSpec.ChainID, moniker)
	if len(crd.Spec.ChainSpec.AdditionalInitArgs) > 0 {
		initCmd += " " + strings.Join(crd.Spec.ChainSpec.AdditionalInitArgs, " ")
	}
	required := []corev1.Container{
		{
			Name:            "clean-init",
			Image:           resolveInfraToolImage(),
			Command:         []string{"sh"},
			Args:            []string{"-c", `rm -rf "$HOME/.tmp/*"`},
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		},
		{
			Name:    chainInitContainer,
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
			Image:           resolveInfraToolImage(),
			Command:         []string{genesisCmd},
			Args:            genesisArgs,
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		},
		{
			Name:            "addrbook-init",
			Image:           resolveInfraToolImage(),
			Command:         []string{addrbookCmd},
			Args:            addrbookArgs,
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		},
		{
			Name:    "config-merge",
			Image:   resolveInfraToolImage(),
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
cp "$OVERLAY_DIR/node_key.json" "$CONFIG_DIR/node_key.json"

echo "Merging config..."
set -x

if [ -f "$TMP_DIR/config.toml" ]; then
	config-merge -f toml "$TMP_DIR/config.toml" "$OVERLAY_DIR/config-overlay.toml" > "$CONFIG_DIR/config.toml"
fi
if [ -f "$TMP_DIR/app.toml" ]; then
	config-merge -f toml "$TMP_DIR/app.toml" "$OVERLAY_DIR/app-overlay.toml" > "$CONFIG_DIR/app.toml"
fi
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
			Image:           resolveInfraToolImage(),
			Command:         []string{cmd},
			Args:            args,
			Env:             env,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		})
	}

	versionCheckCmd := []string{"/manager", "versioncheck"}
	if crd.Spec.ChainSpec.DatabaseBackend != nil {
		versionCheckCmd = append(versionCheckCmd, "-b", *crd.Spec.ChainSpec.DatabaseBackend)
	}

	// Append version check after snapshot download, if applicable.
	// That way the version check will be after the database is initialized.
	// This initContainer will update the crd status with the current height for the pod,
	// And then panic if the image version is not correct for the current height.
	// After the status is patched, the pod will be restarted with the correct image.
	required = append(required, corev1.Container{
		Name:    "version-check",
		Image:   version.Image() + ":" + version.DockerTag(),
		Command: versionCheckCmd,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5m"),
				corev1.ResourceMemory: resource.MustParse("16Mi"),
			},
		},
		Env:             env,
		ImagePullPolicy: tpl.ImagePullPolicy,
		WorkingDir:      workDir,
		SecurityContext: &corev1.SecurityContext{},
	})

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
	if len(crd.Spec.ChainSpec.AdditionalStartArgs) > 0 {
		args = append(args, crd.Spec.ChainSpec.AdditionalStartArgs...)
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

func buildAdditionalPod(
	crd *cosmosv1.CosmosFullNode,
	ordinal int32,
	podSpec cosmosv1.AdditionalPodSpec,
) (*corev1.Pod, error) {
	// Create a unique name for the additional pod
	name := fmt.Sprintf("%s-%d", podSpec.Name, ordinal)

	labels := defaultLabels(crd)
	labels[kube.NameLabel] = appName(crd) + "-" + podSpec.Name

	belongsTo := instanceName(crd, ordinal)

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   crd.Namespace,
			Name:        name,
			Labels:      labels,
			Annotations: make(map[string]string),
		},
		Spec: podSpec.PodSpec,
	}

	if podSpec.PreferSameNode {
		pod.Spec.Affinity = &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									kube.InstanceLabel: belongsTo,
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		}
	}

	// Apply common labels and annotations
	preserveMergeInto(pod.Labels, podSpec.Metadata.Labels)
	preserveMergeInto(pod.Annotations, podSpec.Metadata.Annotations)

	pod.Labels[kube.InstanceLabel] = name
	pod.Labels[kube.BelongsToLabel] = belongsTo

	if len(crd.Spec.ChainSpec.Versions) > 0 {
		instanceHeight := uint64(0)
		if height, ok := crd.Status.Height[belongsTo]; ok {
			instanceHeight = height
		}
		var vrs *cosmosv1.ChainVersion
		for _, v := range crd.Spec.ChainSpec.Versions {
			if instanceHeight < v.UpgradeHeight {
				break
			}
			vrs = &v
		}
		if vrs != nil {
			setVersionedImages(pod, vrs)
		}
	}

	// Handle instance overrides if needed
	if o, ok := crd.Spec.InstanceOverrides[name]; ok {
		if o.DisableStrategy != nil {
			return nil, nil
		}
		if o.Image != "" {
			if len(pod.Spec.Containers) == 0 {
				return nil, fmt.Errorf("no containers in pod %q", name)
			}
			pod.Spec.Containers[0].Image = o.Image
		}
		if o.NodeSelector != nil {
			pod.Spec.NodeSelector = o.NodeSelector
		}
	}

	kube.NormalizeMetadata(&pod.ObjectMeta)
	return pod, nil
}
