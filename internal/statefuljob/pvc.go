package statefuljob

import (
	"fmt"

	cosmosalpha "github.com/bharvest-devops/cosmos-operator/api/v1alpha1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildPVCs builds PVCs given the crd and VolumeSnapshot.
func BuildPVCs(crd *cosmosalpha.StatefulJob, vs *snapshotv1.VolumeSnapshot) ([]*corev1.PersistentVolumeClaim, error) {
	storage, err := findStorage(vs)
	if err != nil {
		return nil, err
	}

	pvc := corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: ptr(crd.Spec.VolumeClaimTemplate.StorageClassName),
			AccessModes:      crd.Spec.VolumeClaimTemplate.AccessModes,
			DataSource: &corev1.TypedLocalObjectReference{
				APIGroup: ptr(vs.GroupVersionKind().Group),
				Kind:     vs.Kind,
				Name:     vs.Name,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: storage},
			},
		},
	}
	pvc.Namespace = crd.Namespace
	pvc.Name = ResourceName(crd)
	pvc.Labels = defaultLabels()

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
