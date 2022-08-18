package fullnode

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
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
			Containers: []corev1.Container{
				{
					Name:  crd.Name,
					Image: "busybox:stable", // TODO: change me to tpl.Image
					// TODO need binary name
					Command:   []string{"/bin/sh"},
					Args:      []string{"-c", `trap : TERM INT; sleep infinity & wait`},
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
	buf := bufPool.Get().(*bytes.Buffer)
	defer buf.Reset()
	defer bufPool.Put(buf)

	enc := json.NewEncoder(buf)
	if err := enc.Encode(crd.Spec.PodTemplate); err != nil {
		panic(err)
	}

	// If chain config changes, we need to trigger a rollout to get new files config files mounted into
	// containers.
	if err := enc.Encode(crd.Spec.ChainConfig); err != nil {
		panic(err)
	}

	h := fnv.New32()
	_, err := h.Write(buf.Bytes())
	if err != nil {
		panic(err)
	}
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
	name := podName(b.crd, ordinal)

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
					LocalObjectReference: corev1.LocalObjectReference{Name: appName(b.crd)},
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

func podName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return kube.ToLabelValue(fmt.Sprintf("%s-%d", appName(crd), ordinal))
}

var fullNodePorts = []corev1.ContainerPort{
	{
		Name:          "api",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: 1317,
	},
	{
		Name:          "rosetta",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: 8080,
	},
	{
		Name:          "grpc",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: 9090,
	},
	{
		Name:          "prometheus",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: 26660,
	},
	{
		Name:          "p2p",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: 26656,
	},
	{
		Name:          "rpc",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: 26657,
	},
	{
		Name:          "web",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: 9091,
	},
}

const (
	workDir      = "/home/operator"
	chainHomeDir = workDir + "/cosmos"
	tmpDir       = workDir + "/.tmp"
	tmpConfigDir = workDir + "/.config"

	infraToolImage = "ghcr.io/strangelove-ventures/infra-toolkit"
)

var (
	securityContext = &corev1.SecurityContext{
		RunAsUser:                ptr(int64(1025)),
		RunAsGroup:               ptr(int64(1025)),
		RunAsNonRoot:             ptr(true),
		AllowPrivilegeEscalation: ptr(false),
	}
	envVars = []corev1.EnvVar{
		{Name: "HOME", Value: workDir},
		{Name: "CHAIN_HOME", Value: chainHomeDir},
		{Name: "GENESIS_FILE", Value: path.Join(chainHomeDir, "config", "genesis.json")},
		{Name: "CONFIG_DIR", Value: path.Join(chainHomeDir, "config")},
	}
)

func initContainers(crd *cosmosv1.CosmosFullNode, moniker string) []corev1.Container {
	tpl := crd.Spec.PodTemplate
	binary := crd.Spec.ChainConfig.Binary
	return []corev1.Container{
		// Chown, so we have proper permissions.
		{
			Name:    "chown",
			Image:   infraToolImage,
			Command: []string{"sh"},
			Args: []string{"-c", `
set -e
chown 1025:1025 "$HOME"
`},
			Env:             envVars,
			ImagePullPolicy: tpl.ImagePullPolicy,
			WorkingDir:      workDir,
			SecurityContext: nil, // Purposefully nil for chown to succeed.
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
	%s init %s --home "$CHAIN_HOME"
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
			SecurityContext: securityContext,
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
			SecurityContext: securityContext,
		},
	}
}
