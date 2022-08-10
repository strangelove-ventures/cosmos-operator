package fullnode

import (
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultAccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
)

// BuildPVCs outputs desired PVCs given the crd.
func BuildPVCs(crd *cosmosv1.CosmosFullNode) []*corev1.PersistentVolumeClaim {
	tpl := crd.Spec.VolumeClaimTemplate
	template := corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: crd.Namespace,
			Labels: map[string]string{
				chainLabel:           kube.ToLabelValue(crd.Name),
				kube.ControllerLabel: kube.ToLabelValue("CosmosFullNode"),
				kube.NameLabel:       kube.ToLabelValue(fmt.Sprintf("%s-fullnode", crd.Name)),
				revisionLabel:        "TODO",
			},
			Annotations: make(map[string]string),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      sliceOrDefault(tpl.AccessModes, defaultAccessModes),
			Resources:        tpl.Resources,
			StorageClassName: tpl.StorageClassName,
			VolumeMode:       valOrDefault(tpl.VolumeMode, ptr(corev1.PersistentVolumeFilesystem)),
		},
	}

	vols := make([]*corev1.PersistentVolumeClaim, crd.Spec.Replicas)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		pvc := template.DeepCopy()

		name := fmt.Sprintf("pvc-%s-fullnode-%d", crd.Name, i)
		pvc.Name = kube.ToName(name)
		pvc.Annotations[OrdinalAnnotation] = kube.ToIntegerValue(i)
		pvc.Labels[kube.InstanceLabel] = kube.ToLabelValue(name)

		vols[i] = pvc
	}
	return vols
}
