package snapshot

import (
	"fmt"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// BuildPVCs builds PVCs given the crd and VolumeSnapshot.
func BuildPVCs(crd *cosmosv1.HostedSnapshot, vs *snapshotv1.VolumeSnapshot) ([]*corev1.PersistentVolumeClaim, error) {
	storage, err := findStorage(vs)
	if err != nil {
		return nil, err
	}

	pvc := corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: ptr(crd.Spec.StorageClassName),
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			DataSource: &corev1.TypedLocalObjectReference{
				APIGroup: ptr(vs.GroupVersionKind().Group),
				Kind:     vs.Kind,
				Name:     vs.Name,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: storage},
			},
		},
	}
	pvc.Namespace = crd.Namespace
	pvc.Name = pvcName(crd)
	pvc.Labels = defaultLabels(crd)

	return []*corev1.PersistentVolumeClaim{&pvc}, nil
}

func findStorage(vs *snapshotv1.VolumeSnapshot) (zero resource.Quantity, _ error) {
	if vs.Status == nil {
		return zero, fmt.Errorf("%s %s: missing status subresource", vs.Kind, vs.Name)
	}
	if vs.Status.RestoreSize == nil {
		return zero, fmt.Errorf("%s %s: missing status.restoreSize", vs.Kind, vs.Name)
	}
	return *vs.Status.RestoreSize, nil
}

func pvcName(crd *cosmosv1.HostedSnapshot) string {
	return kube.ToName("pvc-snapshot-" + crd.Name)
}
