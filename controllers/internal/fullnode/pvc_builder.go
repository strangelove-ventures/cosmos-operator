package fullnode

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"

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
				kube.RevisionLabel:   pvcRevisionHash(crd),
			},
			Annotations: make(map[string]string),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      sliceOrDefault(tpl.AccessModes, defaultAccessModes),
			Resources:        tpl.Resources,
			StorageClassName: ptr(tpl.StorageClassName),
			VolumeMode:       valOrDefault(tpl.VolumeMode, ptr(corev1.PersistentVolumeFilesystem)),
		},
	}

	vols := make([]*corev1.PersistentVolumeClaim, crd.Spec.Replicas)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		pvc := template.DeepCopy()

		name := pvcName(crd.Name, i)
		pvc.Name = name
		pvc.Labels[kube.InstanceLabel] = name
		pvc.Annotations[kube.OrdinalAnnotation] = kube.ToIntegerValue(i)

		vols[i] = pvc
	}
	return vols
}

func pvcName(crdName string, ordinal int32) string {
	name := fmt.Sprintf("pvc-%s-fullnode-%d", crdName, ordinal)
	return kube.ToLabelValue(name)
}

// Attempts to produce a deterministic hash based on the pvc template, so we can detect updates.
// See podRevisionHash for more details.
func pvcRevisionHash(crd *cosmosv1.CosmosFullNode) string {
	buf := bufPool.Get().(*bytes.Buffer)
	defer buf.Reset()
	defer bufPool.Put(buf)

	enc := json.NewEncoder(buf)
	if err := enc.Encode(crd.Spec.VolumeClaimTemplate); err != nil {
		panic(err)
	}
	h := fnv.New32()
	_, err := h.Write(buf.Bytes())
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
