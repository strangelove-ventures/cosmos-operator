package fullnode

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"sort"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultAccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
)

// BuildPVCs outputs desired PVCs given the crd.
func BuildPVCs(crd *cosmosv1.CosmosFullNode) []*corev1.PersistentVolumeClaim {
	tpl := crd.Spec.VolumeClaimTemplate
	base := corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: crd.Namespace,
			Labels: defaultLabels(crd,
				kube.RevisionLabel, pvcRevisionHash(crd),
			),
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
		pvc := base.DeepCopy()

		name := pvcName(crd, i)
		pvc.Name = name
		pvc.Labels[kube.InstanceLabel] = name
		pvc.Annotations[kube.OrdinalAnnotation] = kube.ToIntegerValue(i)

		preserveMergeInto(pvc.Labels, tpl.Metadata.Labels)
		preserveMergeInto(pvc.Annotations, tpl.Metadata.Annotations)

		vols[i] = pvc
	}
	return vols
}

func pvcName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	name := fmt.Sprintf("pvc-%s-%d", appName(crd), ordinal)
	return kube.ToLabelValue(name)
}

// Attempts to produce a deterministic hash based on the pvc template, so we can detect updates.
// See podRevisionHash for more details.
func pvcRevisionHash(crd *cosmosv1.CosmosFullNode) string {
	h := fnv.New32()
	mustWrite(h, mustMarshalJSON(crd.Spec.VolumeClaimTemplate))

	keys := maps.Keys(crd.Spec.InstanceOverrides)
	sort.Strings(keys)
	for _, k := range keys {
		mustWrite(h, mustMarshalJSON(crd.Spec.InstanceOverrides[k].VolumeClaimTemplate))
	}

	return hex.EncodeToString(h.Sum(nil))
}
