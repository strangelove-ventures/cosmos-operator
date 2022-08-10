package fullnode

import (
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
	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "juno"
		crd.Spec.Replicas = 3
		crd.Spec.VolumeClaimTemplate = cosmosv1.CosmosPersistentVolumeClaim{
			StorageClassName: ptr("test-storage-class"),
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceStorage: resource.MustParse("100G")},
			},
		}

		pvcs := BuildPVCs(&crd)
		require.Len(t, pvcs, 3)

		gotNames := lo.Map(pvcs, func(pvc *corev1.PersistentVolumeClaim, _ int) string { return pvc.Name })
		require.Equal(t, []string{"pvc-juno-fullnode-0", "pvc-juno-fullnode-1", "pvc-juno-fullnode-2"}, gotNames)

		gotOrds := lo.Map(pvcs, func(pvc *corev1.PersistentVolumeClaim, _ int) string { return pvc.Annotations[OrdinalAnnotation] })
		require.Equal(t, []string{"0", "1", "2"}, gotOrds)

		revisions := lo.Map(pvcs, func(pvc *corev1.PersistentVolumeClaim, _ int) string { return pvc.Labels[revisionLabel] })
		require.NotEmpty(t, lo.Uniq(revisions))
		require.Len(t, lo.Uniq(revisions), 1)

		for _, got := range pvcs {
			require.Equal(t, crd.Namespace, got.Namespace)
			require.Equal(t, "PersistentVolumeClaim", got.Kind)
			require.Equal(t, "v1", got.APIVersion)

			wantLabels := map[string]string{
				"cosmosfullnode.cosmos.strange.love/chain-name": "juno",
				"app.kubernetes.io/created-by":                  "cosmosfullnode",
				"app.kubernetes.io/name":                        "juno-fullnode",
			}
			// These labels change and tested elsewhere.
			delete(got.Labels, revisionLabel)
			delete(got.Labels, kube.InstanceLabel)

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
		crd.Spec.VolumeClaimTemplate = cosmosv1.CosmosPersistentVolumeClaim{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			VolumeMode:  ptr(corev1.PersistentVolumeBlock),
		}

		pvcs := BuildPVCs(&crd)
		require.Len(t, pvcs, 1)

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
			require.LessOrEqual(t, len(got.Name), 253)
			for _, v := range got.Labels {
				require.LessOrEqual(t, len(v), 63)
			}
			for _, v := range got.Annotations {
				require.LessOrEqual(t, len(v), 63)
			}
		}
	})
}