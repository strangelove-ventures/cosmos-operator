package fullnode

import (
	"errors"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: crd.Namespace,
			Labels: map[string]string{
				chainLabel:           kube.ToLabelValue(crd.Name),
				kube.ControllerLabel: kube.ToLabelValue("CosmosFullNode"),
				kube.NameLabel:       kube.ToLabelValue(fmt.Sprintf("%s-fullnode", crd.Name)),
				kube.VersionLabel:    kube.ParseImageVersion(crd.Spec.Image),
			},
			Annotations: make(map[string]string), // initialized because other methods use this map
		},
		Spec: corev1.PodSpec{
			Volumes:                       nil, // TODO: must create volumes before this step
			InitContainers:                nil, // TODO: real chain will need init containers
			TerminationGracePeriodSeconds: ptr(int64(30)),
			Containers: []corev1.Container{
				{
					Name:  crd.Name,
					Image: crd.Spec.Image,
					// TODO need binary name
					Command: []string{"sleep"},
					Args:    []string{"infinity"},
					Ports:   fullNodePorts,
					// TODO (nix - 7/27/22) - Set these values.
					Resources:      corev1.ResourceRequirements{},
					VolumeMounts:   nil,
					LivenessProbe:  nil,
					ReadinessProbe: nil,
					StartupProbe:   nil,

					// Purposefully blank. Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
					ImagePullPolicy: "",
				},
			},
		},
	}
	return PodBuilder{
		crd: crd,
		pod: &pod,
	}
}

// Build assigns the CosmosFullNode crd as the owner and returns a fully constructed pod.
func (b PodBuilder) Build() *corev1.Pod {
	return b.pod
}

// WithOrdinal updates adds name and other metadata to the pod using "ordinal" which is the pod's
// ordered sequence. Pods have deterministic, consistent names similar to a StatefulSet instead of generated names.
func (b PodBuilder) WithOrdinal(ordinal int32) PodBuilder {
	pod := b.pod.DeepCopy()
	name := b.name(ordinal)

	pod.Annotations[kube.OrdinalAnnotation] = kube.ToIntegerValue(ordinal)
	pod.Labels[kube.InstanceLabel] = kube.ToLabelValue(name)

	pod.Name = kube.ToName(name)

	b.pod = pod
	return b
}

func (b PodBuilder) name(ordinal int32) string {
	return fmt.Sprintf("%s-fullnode-%d", b.crd.Name, ordinal)
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
