package fullnode

import (
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"gopkg.in/inf.v0"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	snapshotGrowthFactor = 102
)

var (
	defaultAccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
)

// BuildPVCs outputs desired PVCs given the crd.
func BuildPVCs(crd *cosmosv1.CosmosFullNode) []diff.Resource[*corev1.PersistentVolumeClaim] {
	base := corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   crd.Namespace,
			Labels:      defaultLabels(crd),
			Annotations: make(map[string]string),
		},
	}

	var pvcs []diff.Resource[*corev1.PersistentVolumeClaim]
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		if pvcDisabled(crd, i) {
			continue
		}

		pvc := base.DeepCopy()
		name := pvcName(crd, i)
		pvc.Name = name
		pvc.Labels[kube.InstanceLabel] = instanceName(crd, i)

		tpl := crd.Spec.VolumeClaimTemplate
		if override, ok := crd.Spec.InstanceOverrides[instanceName(crd, i)]; ok {
			if overrideTpl := override.VolumeClaimTemplate; overrideTpl != nil {
				tpl = *overrideTpl
			}
		}

		pvc.Spec = corev1.PersistentVolumeClaimSpec{
			AccessModes:      sliceOrDefault(tpl.AccessModes, defaultAccessModes),
			Resources:        pvcResources(crd),
			StorageClassName: ptr(tpl.StorageClassName),
			VolumeMode:       valOrDefault(tpl.VolumeMode, ptr(corev1.PersistentVolumeFilesystem)),
			DataSource:       tpl.DataSource,
		}

		preserveMergeInto(pvc.Labels, tpl.Metadata.Labels)
		preserveMergeInto(pvc.Annotations, tpl.Metadata.Annotations)
		kube.NormalizeMetadata(&pvc.ObjectMeta)

		pvcs = append(pvcs, diff.Adapt(pvc, i))
	}
	return pvcs
}

func pvcDisabled(crd *cosmosv1.CosmosFullNode, ordinal int32) bool {
	name := instanceName(crd, ordinal)
	disable := crd.Spec.InstanceOverrides[name].DisableStrategy
	return disable != nil && *disable == cosmosv1.DisableAll
}

func pvcName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	name := fmt.Sprintf("pvc-%s-%d", appName(crd), ordinal)
	return kube.ToName(name)
}

func pvcResources(crd *cosmosv1.CosmosFullNode) corev1.ResourceRequirements {
	var (
		reqs = crd.Spec.VolumeClaimTemplate.Resources
		size = reqs.Requests[corev1.ResourceStorage]
	)

	if autoScale := crd.Status.SelfHealing.PVCAutoScale; autoScale != nil {
		requestedSize := autoScale.RequestedSize.DeepCopy()
		newSize := requestedSize.AsDec()
		sizeWithPadding := resource.NewDecimalQuantity(*newSize.Mul(newSize, inf.NewDec(snapshotGrowthFactor, 2)), resource.DecimalSI)
		if sizeWithPadding.Cmp(size) > 0 {
			reqs.Requests[corev1.ResourceStorage] = *sizeWithPadding
		}
	}
	return reqs
}
