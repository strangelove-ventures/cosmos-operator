package fullnode

import (
	"errors"
	"fmt"
	"strconv"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
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
			Namespace:   crd.Namespace,
			Annotations: nil, // TODO: expose prom metrics
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
					// TODO probably need the below
					Ports:          nil,
					EnvFrom:        nil,
					Env:            nil,
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
	pod.Labels = b.labels(ordinal)
	pod.Name = b.name(ordinal)
	b.pod = pod
	return b
}

func (b PodBuilder) labels(ordinal int32) map[string]string {
	return map[string]string{
		chainLabel:   b.crd.Name,
		OrdinalLabel: strconv.FormatInt(int64(ordinal), 10),
	}
}

func (b PodBuilder) name(ordinal int32) string {
	return fmt.Sprintf("%s-%d", b.crd.Name, ordinal)
}
