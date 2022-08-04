package fullnode

import (
	"errors"
	"fmt"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodBuilder builds corev1.Pods
type PodBuilder struct {
	current []*corev1.Pod

	crd      *cosmosv1.CosmosFullNode
	template corev1.Pod
}

// NewPodBuilder returns a valid PodBuilder.
//
// Panics if any argument is nil.
func NewPodBuilder(crd *cosmosv1.CosmosFullNode, current []*corev1.Pod) PodBuilder {
	if crd == nil {
		panic(errors.New("nil CosmosFullNode"))
	}
	template := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			// Needs initialization because other builder methods may add to it.
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			InitContainers: nil, // TODO
			// Later methods expect slice to be valid
			Containers: make([]corev1.Container, 1),
		},
	}
	return PodBuilder{
		crd:      crd,
		template: template,
		current:  current,
	}
}

// Build assigns the CosmosFullNode crd as the owner and returns a fully constructed pod.
func (b PodBuilder) Build() *corev1.Pod {
	// TODO
	if b.template.Name == "" {
		panic(errors.New("must call WithOrdinal to set pod name"))
	}

	pod, ok := lo.Find(b.current, func(pod *corev1.Pod) bool { return b.template.Name == pod.Name })
	if !ok {
		fmt.Println("****DID NOT FIND POD*********")
		pod = &b.template
	} else {
		fmt.Println("****USING EXISTING POD******")
	}

	// TODO
	//if b.crd.Namespace != pod.Namespace {
	//	panic(errors.New("namespace mismatch "))
	//}

	pod.Namespace = b.crd.Namespace
	pod.Name = b.template.Name

	pod.Labels = b.template.Labels
	pod.Labels[chainLabel] = kube.ToLabelValue(b.crd.Name)
	pod.Labels[kube.ControllerLabel] = kube.ToLabelValue("CosmosFullNode")
	pod.Labels[kube.NameLabel] = kube.ToLabelValue(fmt.Sprintf("%s-fullnode", b.crd.Name))
	pod.Labels[kube.VersionLabel] = kube.ParseImageVersion(b.crd.Spec.Image)

	// TODO (nix - 8/4/22) Volumes, init containers, resources, probes, prom annotations

	pod.Spec.TerminationGracePeriodSeconds = ptr(int64(30))

	startContainer := pod.Spec.Containers[0]
	startContainer.Name = "fullnode"
	startContainer.Image = b.crd.Spec.Image
	startContainer.Command = []string{"sleep"}
	startContainer.Args = []string{"infinity"}
	startContainer.Ports = fullNodePorts
	startContainer.Image = b.crd.Spec.Image

	pod.Spec.Containers[0] = startContainer

	//pod := corev1.Pod{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Namespace: crd.Namespace,
	//		Labels: map[string]string{
	//			chainLabel:           kube.ToLabelValue(crd.Name),
	//			kube.ControllerLabel: kube.ToLabelValue("CosmosFullNode"),
	//			kube.NameLabel:       kube.ToLabelValue(fmt.Sprintf("%s-fullnode", crd.Name)),
	//			kube.VersionLabel:    kube.ParseImageVersion(crd.Spec.Image),
	//		},
	//		// Needs initialized map because other builder methods may add to it.
	//		Annotations: make(map[string]string), // TODO (nix - 8/2/22) Prom metrics
	//	},
	//	Spec: corev1.PodSpec{
	//		Volumes:                       nil, // TODO: must create volumes before this step
	//		InitContainers:                nil, // TODO: real chain will need init containers
	//		TerminationGracePeriodSeconds: ptr(int64(30)),
	//		Containers: []corev1.Container{
	//			{
	//				Name:  crd.Name,
	//				Image: crd.Spec.Image,
	//				// TODO need binary name
	//				Command: []string{"sleep"},
	//				Args:    []string{"infinity"},
	//				Ports:   fullNodePorts,
	//				// TODO (nix - 7/27/22) - Set these values.
	//				Resources:      corev1.ResourceRequirements{},
	//				VolumeMounts:   nil,
	//				LivenessProbe:  nil,
	//				ReadinessProbe: nil,
	//				StartupProbe:   nil,
	//			},
	//		},
	//	},
	//}

	return pod
}

// WithOrdinal updates adds name and other metadata to the pod using "ordinal" which is the pod's
// ordered sequence. Pods have deterministic, consistent names similar to a StatefulSet instead of generated names.
func (b PodBuilder) WithOrdinal(ordinal int32) PodBuilder {
	pod := b.template

	name := b.name(ordinal)
	pod.Labels[kube.InstanceLabel] = kube.ToLabelValue(name)
	pod.Annotations[kube.OrdinalAnnotation] = kube.ToIntegerValue(ordinal)
	pod.Name = kube.ToName(name)

	b.template = pod
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
