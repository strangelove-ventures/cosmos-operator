package fullnode

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"path"
	"sync"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

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

	tpl := crd.Spec.PodTemplate

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
			InitContainers:                nil, // TODO: real chain will need init containers
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
			},
			Containers: []corev1.Container{
				{
					Name:  crd.Name,
					Image: tpl.Image,
					// The following is a useful hack if you need to inspect the PV.
					//Command: []string{"/bin/sh"},
					//Args:    []string{"-c", `trap : TERM INT; sleep infinity & wait`},
					Command:   []string{crd.Spec.ChainConfig.Binary},
					Args:      startCommandArgs(crd.Spec.ChainConfig),
					Env:       envVars,
					Ports:     fullNodePorts,
					Resources: tpl.Resources,
					// TODO (nix - 7/27/22) - Set these values.
					LivenessProbe:  nil,
					ReadinessProbe: nil,
					StartupProbe:   nil,

					ImagePullPolicy: tpl.ImagePullPolicy,
					WorkingDir:      workDir,
				},
			},
		},
	}

	// Conditionally add custom labels and annotations, preserving key/values already set.
	for k, v := range tpl.Metadata.Labels {
		_, ok := pod.ObjectMeta.Labels[k]
		if !ok {
			pod.ObjectMeta.Labels[k] = kube.ToLabelValue(v)
		}
	}
	for k, v := range tpl.Metadata.Annotations {
		_, ok := pod.ObjectMeta.Annotations[k]
		if !ok {
			pod.ObjectMeta.Annotations[k] = kube.ToLabelValue(v)
		}
	}

	return PodBuilder{
		crd: crd,
		pod: &pod,
	}
}

// Attempts to produce a deterministic hash based on the pod template, so we can detect updates.
// encoding/gob was used at first but proved non-deterministic. JSON by nature is unordered, however thousands
// of fuzz tests showed encoding/json to be deterministic. There are other json packages like jsoniter that sort keys
// if stdlib encoding/json ever becomes a problem.
func podRevisionHash(crd *cosmosv1.CosmosFullNode) string {
	h := fnv.New32()
	mustWrite(h, mustMarshalJSON(crd.Spec.PodTemplate))
	mustWrite(h, mustMarshalJSON(crd.Spec.ChainConfig))
	return hex.EncodeToString(h.Sum(nil))
}

// Build assigns the CosmosFullNode crd as the owner and returns a fully constructed pod.
func (b PodBuilder) Build() *corev1.Pod {
	return b.pod
}

// WithOrdinal updates adds name and other metadata to the pod using "ordinal" which is the pod's
// ordered sequence. Pods have deterministic, consistent names similar to a StatefulSet instead of generated names.
func (b PodBuilder) WithOrdinal(ordinal int32) PodBuilder {
	pod := b.pod.DeepCopy()
	name := instanceName(b.crd, ordinal)

	pod.Annotations[kube.OrdinalAnnotation] = kube.ToIntegerValue(ordinal)
	pod.Labels[kube.InstanceLabel] = kube.ToLabelValue(name)

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

	infraToolImage = "ghcr.io/strangelove-ventures/infra-toolkit"
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
	binary := crd.Spec.ChainConfig.Binary
	genesisCmd, genesisArgs := DownloadGenesisCommand(crd.Spec.ChainConfig)

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
	%s init %s --home "$CHAIN_HOME"
	# Remove because downstream containers check the presence of this file.
	rm "$GENESIS_FILE"
else
	echo "Skipping chain init; already initialized."
fi

echo "Initializing into tmp dir for downstream processing..."
%s init %s --home "$HOME/.tmp"
`, binary, moniker, binary, moniker),
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
		cmd, args := DownloadSnapshotCommand(crd.Spec.ChainConfig)
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

func startCommandArgs(cfg cosmosv1.ChainConfig) []string {
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
	return crd.Spec.ChainConfig.SnapshotURL != nil || crd.Spec.ChainConfig.SnapshotScript != nil
}
