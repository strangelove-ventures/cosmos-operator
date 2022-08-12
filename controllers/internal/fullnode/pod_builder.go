package fullnode

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
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
			Labels: map[string]string{
				kube.ControllerLabel: kube.ToLabelValue("CosmosFullNode"),
				kube.NameLabel:       appName(crd),
				kube.VersionLabel:    kube.ParseImageVersion(tpl.Image),
				revisionLabel:        podRevisionHash(crd),
			},
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
					Image: tpl.Image,
					// TODO need binary name
					Command:   []string{"/bin/sh"},
					Args:      []string{"-c", `trap : TERM INT; sleep infinity & wait`},
					Ports:     fullNodePorts,
					Resources: tpl.Resources,
					// TODO (nix - 7/27/22) - Set these values.
					LivenessProbe:  nil,
					ReadinessProbe: nil,
					StartupProbe:   nil,

					ImagePullPolicy: tpl.ImagePullPolicy,
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
	name := podName(b.crd.Name, ordinal)

	pod.Annotations[OrdinalAnnotation] = kube.ToIntegerValue(ordinal)
	pod.Labels[kube.InstanceLabel] = name

	pod.Name = name

	volName := kube.ToName(fmt.Sprintf("vol-%s-fullnode-%d", b.crd.Name, ordinal))
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName(b.crd.Name, ordinal)},
			},
		},
	}
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = []corev1.VolumeMount{
			{Name: volName, MountPath: "/home/cosmos"}, // TODO (nix - 8/12/22) MountPath may not be correct.
		}
	}

	b.pod = pod
	return b
}

func appName(crd *cosmosv1.CosmosFullNode) string {
	return kube.ToLabelValue(fmt.Sprintf("%s-fullnode", crd.Name))
}

func podName(crdName string, ordinal int32) string {
	return kube.ToLabelValue(fmt.Sprintf("%s-fullnode-%d", crdName, ordinal))
}

func defaultPodAffinity(crdName string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{kube.NameLabel: "osmosis-fullnode"},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}
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
