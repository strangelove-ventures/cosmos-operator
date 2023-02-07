package fullnode

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"path"
	"strings"
	"sync"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

const healthCheckPort = 1251

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
			Namespace: crd.Namespace,
			Labels: defaultLabels(crd,
				kube.RevisionLabel, podRevisionHash(crd),
			),
			// TODO: prom metrics
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: valOrDefault(tpl.TerminationGracePeriodSeconds, ptr(int64(30))),
			Affinity:                      tpl.Affinity,
			NodeSelector:                  tpl.NodeSelector,
			Tolerations:                   tpl.Tolerations,
			PriorityClassName:             tpl.PriorityClassName,
			Priority:                      tpl.Priority,
			ImagePullSecrets:              tpl.ImagePullSecrets,
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
					Env:             envVars,
					Ports:           buildPorts(crd.Spec.Type),
					Resources:       tpl.Resources,
					ReadinessProbe:  probes[0],
					ImagePullPolicy: tpl.ImagePullPolicy,
					WorkingDir:      workDir,
				},
				// Healtcheck sidecar to ensure pod is in sync with the chain.
				{
					Name:    "healthcheck",
					Image:   "ghcr.io/strangelove-ventures/ignite-health-check:v0.0.1",
					Command: []string{"ihc"},
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

	preserveMergeInto(pod.Labels, tpl.Metadata.Labels)
	preserveMergeInto(pod.Annotations, tpl.Metadata.Annotations)

	return PodBuilder{
		crd: crd,
		pod: &pod,
	}
}

func podReadinessProbes(crd *cosmosv1.CosmosFullNode) []*corev1.Probe {
	if crd.Spec.Type == cosmosv1.FullNodeSentry {
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

// Attempts to produce a deterministic hash based on the pod template, so we can detect updates.
// encoding/gob was used at first but proved non-deterministic. JSON by nature is unordered, however thousands
// of fuzz tests showed encoding/json to be deterministic. There are other json packages like jsoniter that sort keys
// if stdlib encoding/json ever becomes a problem.
func podRevisionHash(crd *cosmosv1.CosmosFullNode) string {
	h := fnv.New32()
	mustWrite(h, mustMarshalJSON(crd.Spec.PodTemplate))
	mustWrite(h, mustMarshalJSON(crd.Spec.ChainSpec))
	mustWrite(h, mustMarshalJSON(crd.Spec.Type))
	return hex.EncodeToString(h.Sum(nil))
}

// Build assigns the CosmosFullNode crd as the owner and returns a fully constructed pod.
func (b PodBuilder) Build() *corev1.Pod {
	kube.NormalizeMetadata(&b.pod.ObjectMeta)
	return b.pod
}

// WithOrdinal updates adds name and other metadata to the pod using "ordinal" which is the pod's
// ordered sequence. Pods have deterministic, consistent names similar to a StatefulSet instead of generated names.
func (b PodBuilder) WithOrdinal(ordinal int32) PodBuilder {
	pod := b.pod.DeepCopy()
	name := instanceName(b.crd, ordinal)

	pod.Annotations[kube.OrdinalAnnotation] = kube.ToIntegerValue(ordinal)
	pod.Labels[kube.InstanceLabel] = name

	pod.Name = name
	pod.Spec.InitContainers = initContainers(b.crd, name)

	const (
		volChainHome = "vol-chain-home" // Stores live chain data and config files.
		volTmp       = "vol-tmp"        // Stores temporary config files for manipulation later.
		volConfig    = "vol-config"     // Items from ConfigMap.
	)
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
	}

	mounts := []corev1.VolumeMount{
		{Name: volChainHome, MountPath: chainHomeDir},
	}
	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].VolumeMounts = append(mounts, []corev1.VolumeMount{
			{Name: volTmp, MountPath: tmpDir},
			{Name: volConfig, MountPath: tmpConfigDir},
		}...)
	}
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = mounts
	}

	b.pod = pod
	return b
}

const (
	workDir      = "/home/operator"
	chainHomeDir = workDir + "/cosmos"
	tmpDir       = workDir + "/.tmp"
	tmpConfigDir = workDir + "/.config"

	infraToolImage = "ghcr.io/strangelove-ventures/infra-toolkit:v0.0.1"
)

var (
	envVars = []corev1.EnvVar{
		{Name: "HOME", Value: workDir},
		{Name: "CHAIN_HOME", Value: chainHomeDir},
		{Name: "GENESIS_FILE", Value: path.Join(chainHomeDir, "config", "genesis.json")},
		{Name: "CONFIG_DIR", Value: path.Join(chainHomeDir, "config")},
		{Name: "DATA_DIR", Value: path.Join(chainHomeDir, "data")},
	}
)

func initContainers(crd *cosmosv1.CosmosFullNode, moniker string) []corev1.Container {
	tpl := crd.Spec.PodTemplate
	binary := crd.Spec.ChainSpec.Binary
	genesisCmd, genesisArgs := DownloadGenesisCommand(crd.Spec.ChainSpec)

	initCmd := fmt.Sprintf("%s init %s --chain-id %s", binary, moniker, crd.Spec.ChainSpec.ChainID)
	required := []corev1.Container{
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
	# Remove because downstream containers check the presence of this file.
	rm "$GENESIS_FILE"
else
	echo "Skipping chain init; already initialized."
fi

echo "Initializing into tmp dir for downstream processing..."
%s --home "$HOME/.tmp"
`, initCmd, initCmd),
			},
			Env:             envVars,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		},

		{
			Name:            "genesis-init",
			Image:           infraToolImage,
			Command:         []string{genesisCmd},
			Args:            genesisArgs,
			Env:             envVars,
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
echo "Merging config..."
set -x
config-merge -f toml "$TMP_DIR/config.toml" "$OVERLAY_DIR/config-overlay.toml" > "$CONFIG_DIR/config.toml"
config-merge -f toml "$TMP_DIR/app.toml" "$OVERLAY_DIR/app-overlay.toml" > "$CONFIG_DIR/app.toml"
`,
			},
			Env:             envVars,
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
			Env:             envVars,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
		})
	}

	return required
}

func startCmdAndArgs(crd *cosmosv1.CosmosFullNode) (string, []string) {
	var (
		binary             = crd.Spec.ChainSpec.Binary
		args               = startCommandArgs(crd.Spec.ChainSpec)
		privvalSleep int32 = 10
	)
	if v := crd.Spec.ChainSpec.PrivvalSleepSeconds; v != nil {
		privvalSleep = *v
	}

	if crd.Spec.Type == cosmosv1.FullNodeSentry && privvalSleep > 0 {
		shellBody := fmt.Sprintf(`sleep %d
%s %s`, privvalSleep, binary, strings.Join(args, " "))
		return "sh", []string{"-c", shellBody}
	}

	return binary, args
}

func startCommandArgs(cfg cosmosv1.ChainSpec) []string {
	args := []string{"start", "--home", chainHomeDir}
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

// PVCName returns the primary PVC holding the chain data associated with the pod.
func PVCName(pod *corev1.Pod) string {
	if vols := pod.Spec.Volumes; len(vols) > 0 {
		if claim := pod.Spec.Volumes[0].PersistentVolumeClaim; claim != nil {
			return claim.ClaimName
		}
	}
	return ""
}
