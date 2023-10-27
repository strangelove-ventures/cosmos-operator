package fullnode

import (
	"fmt"
	"strings"
	"testing"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/strangelove-ventures/cosmos-operator/internal/test"
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
		crd.Spec.VolumeClaimTemplate.StorageClassName = "test-storage-class"

		crd.Spec.InstanceOverrides = map[string]cosmosv1.InstanceOverridesSpec{
			"juno-0": {},
		}

		initial := BuildPVCs(&crd, map[int32]*dataSource{}, nil)
		for i, r := range initial {
			require.Equal(t, int64(i), r.Ordinal())
			require.NotEmpty(t, r.Revision())
		}

		initialPVCs := lo.Map(initial, func(r diff.Resource[*corev1.PersistentVolumeClaim], _ int) *corev1.PersistentVolumeClaim {
			return r.Object()
		})

		pvcs := lo.Map(BuildPVCs(&crd, map[int32]*dataSource{}, initialPVCs), func(r diff.Resource[*corev1.PersistentVolumeClaim], _ int) *corev1.PersistentVolumeClaim {
			return r.Object()
		})

		require.Len(t, pvcs, 3)

		gotNames := lo.Map(pvcs, func(pvc *corev1.PersistentVolumeClaim, _ int) string { return pvc.Name })
		require.Equal(t, []string{"pvc-juno-0", "pvc-juno-1", "pvc-juno-2"}, gotNames)

		for i, got := range pvcs {
			require.Equal(t, crd.Namespace, got.Namespace)
			require.Equal(t, "PersistentVolumeClaim", got.Kind)
			require.Equal(t, "v1", got.APIVersion)

			wantLabels := map[string]string{
				"app.kubernetes.io/created-by": "cosmos-operator",
				"app.kubernetes.io/component":  "CosmosFullNode",
				"app.kubernetes.io/name":       "juno",
				"app.kubernetes.io/instance":   fmt.Sprintf("juno-%d", i),
				"app.kubernetes.io/version":    "v1.2.3",
				"cosmos.strange.love/network":  "mainnet",
				"cosmos.strange.love/type":     "FullNode",
			}
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
		crd.Spec.VolumeClaimTemplate.Metadata = cosmosv1.Metadata{
			Labels:      map[string]string{"label": "value", "app.kubernetes.io/created-by": "should not see me"},
			Annotations: map[string]string{"annot": "value"},
		}
		crd.Spec.VolumeClaimTemplate.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		crd.Spec.VolumeClaimTemplate.VolumeMode = ptr(corev1.PersistentVolumeBlock)
		crd.Spec.VolumeClaimTemplate.DataSource = &corev1.TypedLocalObjectReference{
			Kind: "TestKind",
			Name: "source-name",
		}

		pvcs := BuildPVCs(&crd, map[int32]*dataSource{
			0: {
				ref: crd.Spec.VolumeClaimTemplate.DataSource,
			},
		}, nil)
		require.NotEmpty(t, pvcs)

		got := pvcs[0].Object()
		require.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, got.Spec.AccessModes)
		require.Equal(t, corev1.PersistentVolumeBlock, *got.Spec.VolumeMode)

		require.Equal(t, "value", got.Annotations["annot"])

		require.Equal(t, "cosmos-operator", got.Labels[kube.ControllerLabel])
		require.Equal(t, "value", got.Labels["label"])

		require.Equal(t, crd.Spec.VolumeClaimTemplate.DataSource, got.Spec.DataSource)
	})

	t.Run("instance override", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "cosmoshub"
		crd.Spec.Replicas = 3
		crd.Spec.InstanceOverrides = map[string]cosmosv1.InstanceOverridesSpec{
			"cosmoshub-0": {
				VolumeClaimTemplate: &cosmosv1.PersistentVolumeClaimSpec{
					StorageClassName: "override",
				},
			},
			"cosmoshub-1": {
				DisableStrategy: ptr(cosmosv1.DisableAll),
			},
			"cosmoshub-2": {
				DisableStrategy: ptr(cosmosv1.DisablePod),
			},
			"does-not-exist": {
				VolumeClaimTemplate: &cosmosv1.PersistentVolumeClaimSpec{
					StorageClassName: "should never see me",
				},
			},
		}

		pvcs := BuildPVCs(&crd, map[int32]*dataSource{}, nil)
		require.Equal(t, 2, len(pvcs))

		got1, got2 := pvcs[0].Object(), pvcs[1].Object()

		require.NotEqual(t, got1.Spec, got2.Spec)
		require.Equal(t, []string{"pvc-cosmoshub-0", "pvc-cosmoshub-2"}, []string{got1.Name, got2.Name})
		require.Equal(t, "override", *got1.Spec.StorageClassName)
	})

	t.Run("long names", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = strings.Repeat("Y", 300)

		pvcs := BuildPVCs(&crd, map[int32]*dataSource{}, nil)
		require.NotEmpty(t, pvcs)

		for _, got := range pvcs {
			test.RequireValidMetadata(t, got.Object())
		}
	})

	t.Run("pvc auto scale with padding", func(t *testing.T) {
		t.Run("given auto scale size less then current size", func(t *testing.T) {
			for _, tt := range []struct {
				SpecQuant, AutoScaleQuant, WantQuant string
			}{
				{"100G", "97G", "100G"},
			} {
				crd := defaultCRD()
				crd.Spec.Replicas = 1
				crd.Spec.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(tt.SpecQuant)

				crd.Status.SelfHealing.PVCAutoScale = map[string]*cosmosv1.PVCAutoScaleStatus{
					"pvc-osmosis-0": {
						RequestedSize: resource.MustParse(tt.AutoScaleQuant),
					},
				}

				pvcs := BuildPVCs(&crd, map[int32]*dataSource{}, nil)
				require.Len(t, pvcs, 1, tt)

				want := corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(tt.WantQuant)}
				require.Equal(t, want.Storage().Value(), pvcs[0].Object().Spec.Resources.Requests.Storage().Value(), tt)
			}
		})

		t.Run("given auto scale size equal to current size", func(t *testing.T) {
			for _, tt := range []struct {
				SpecQuant, AutoScaleQuant, WantQuant string
			}{
				{"102G", "100G", "102G"},
			} {
				crd := defaultCRD()
				crd.Spec.Replicas = 1
				crd.Spec.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(tt.SpecQuant)

				crd.Status.SelfHealing.PVCAutoScale = map[string]*cosmosv1.PVCAutoScaleStatus{
					"pvc-osmosis-0": {
						RequestedSize: resource.MustParse(tt.AutoScaleQuant),
					},
				}

				pvcs := BuildPVCs(&crd, map[int32]*dataSource{}, nil)
				require.Len(t, pvcs, 1, tt)

				want := corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(tt.WantQuant)}
				require.Equal(t, want, pvcs[0].Object().Spec.Resources.Requests, tt)
			}
		})

		t.Run("given auto scale size greater than current size", func(t *testing.T) {
			for _, tt := range []struct {
				SpecQuant, AutoScaleQuant, WantQuant string
			}{
				{"100G", "100G", "102G"},
			} {
				crd := defaultCRD()
				crd.Spec.Replicas = 1
				crd.Spec.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(tt.SpecQuant)

				crd.Status.SelfHealing.PVCAutoScale = map[string]*cosmosv1.PVCAutoScaleStatus{
					"pvc-osmosis-0": {
						RequestedSize: resource.MustParse(tt.AutoScaleQuant),
					},
				}

				pvcs := BuildPVCs(&crd, map[int32]*dataSource{}, nil)
				require.Len(t, pvcs, 1, tt)

				want := corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(tt.WantQuant)}
				require.Equal(t, want.Storage().Value(), pvcs[0].Object().Spec.Resources.Requests.Storage().Value(), tt)
			}
		})
	})

	test.HasTypeLabel(t, func(crd cosmosv1.CosmosFullNode) []map[string]string {
		pvcs := BuildPVCs(&crd, map[int32]*dataSource{}, nil)
		labels := make([]map[string]string, 0)
		for _, pvc := range pvcs {
			labels = append(labels, pvc.Object().Labels)
		}
		return labels
	})
}
