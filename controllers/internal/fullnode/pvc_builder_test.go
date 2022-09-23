package fullnode

import (
	"fmt"
	"strings"
	"testing"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestBuildPVCs(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "juno"
		crd.Spec.Replicas = 3
		crd.Spec.VolumeClaimTemplate = cosmosv1.PersistentVolumeClaimSpec{
			StorageClassName: "test-storage-class",
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceStorage: resource.MustParse("100G")},
			},
		}

		pvcs := BuildPVCs(&crd)
		require.Len(t, pvcs, 3)

		gotNames := lo.Map(pvcs, func(pvc *corev1.PersistentVolumeClaim, _ int) string { return pvc.Name })
		require.Equal(t, []string{"pvc-juno-0", "pvc-juno-1", "pvc-juno-2"}, gotNames)

		gotOrds := lo.Map(pvcs, func(pvc *corev1.PersistentVolumeClaim, _ int) string { return pvc.Annotations[kube.OrdinalAnnotation] })
		require.Equal(t, []string{"0", "1", "2"}, gotOrds)

		revisions := lo.Map(pvcs, func(pvc *corev1.PersistentVolumeClaim, _ int) string { return pvc.Labels[kube.RevisionLabel] })
		require.NotEmpty(t, lo.Uniq(revisions))
		require.Len(t, lo.Uniq(revisions), 1)

		for i, got := range pvcs {
			require.Equal(t, crd.Namespace, got.Namespace)
			require.Equal(t, "PersistentVolumeClaim", got.Kind)
			require.Equal(t, "v1", got.APIVersion)

			wantLabels := map[string]string{
				"app.kubernetes.io/created-by": "cosmosfullnode",
				"app.kubernetes.io/name":       "juno",
				"app.kubernetes.io/instance":   fmt.Sprintf("juno-%d", i),
				"app.kubernetes.io/version":    "v1.2.3",
				"cosmos.strange.love/network":  "mainnet",
			}
			// These labels change and tested elsewhere.
			delete(got.Labels, kube.RevisionLabel)

			require.Equal(t, wantLabels, got.Labels)

			require.Len(t, got.Spec.AccessModes, 1)
			require.Equal(t, corev1.ReadWriteOnce, got.Spec.AccessModes[0])

			require.Equal(t, crd.Spec.VolumeClaimTemplate.Resources, got.Spec.Resources)
			require.Equal(t, "test-storage-class", *got.Spec.StorageClassName)
			require.Equal(t, corev1.PersistentVolumeFilesystem, *got.Spec.VolumeMode)
		}
	})

	t.Run("advanced configuration", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 1
		crd.Spec.VolumeClaimTemplate = cosmosv1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			VolumeMode:  ptr(corev1.PersistentVolumeBlock),
		}

		pvcs := BuildPVCs(&crd)
		require.NotEmpty(t, pvcs)

		got := pvcs[0]
		require.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, got.Spec.AccessModes)
		require.Equal(t, corev1.PersistentVolumeBlock, *got.Spec.VolumeMode)
	})

	t.Run("long names", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = strings.Repeat("Y", 300)

		pvcs := BuildPVCs(&crd)
		require.NotEmpty(t, pvcs)

		for _, got := range pvcs {
			RequireValidMetadata(t, got)
		}
	})
}

func FuzzBuildPVCs(f *testing.F) {
	crd := defaultCRD()
	crd.Spec.Replicas = 1

	f.Add("premium-rwo", "storage")
	f.Fuzz(func(t *testing.T, storageClass, resourceKey string) {
		crd.Spec.VolumeClaimTemplate.StorageClassName = storageClass
		crd.Spec.VolumeClaimTemplate.Resources = corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceName(resourceKey): resource.MustParse("100G"),
			},
		}

		pvc1 := BuildPVCs(&crd)[0]
		pvc2 := BuildPVCs(&crd)[0]

		require.NotEmpty(t, pvc1.Labels[kube.RevisionLabel])
		require.NotEmpty(t, pvc2.Labels[kube.RevisionLabel])

		require.Equal(t, pvc1.Labels[kube.RevisionLabel], pvc2.Labels[kube.RevisionLabel])

		crd.Spec.VolumeClaimTemplate.StorageClassName = "different"

		pvc3 := BuildPVCs(&crd)[0]
		require.NotEmpty(t, pvc3.Labels[kube.RevisionLabel])
		require.NotEqual(t, pvc3.Labels[kube.RevisionLabel], pvc1.Labels[kube.RevisionLabel])
	})
}
