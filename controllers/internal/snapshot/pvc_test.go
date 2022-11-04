package snapshot

import (
	"testing"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildPVCs(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		crd := cosmosv1.HostedSnapshot{
			Spec: cosmosv1.HostedSnapshotSpec{
				VolumeClaimTemplate: cosmosv1.SnapshotVolumeClaimTemplate{
					StorageClassName: "primo",
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOncePod},
				},
			},
		}
		crd.Name = "my-test"
		crd.Namespace = "test"

		vs := snapshotv1.VolumeSnapshot{
			TypeMeta: metav1.TypeMeta{
				Kind:       "VolumeSnapshot",
				APIVersion: "snapshot.storage.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-snapshot",
			},
			Status: &snapshotv1.VolumeSnapshotStatus{
				RestoreSize: ptr(resource.MustParse("10Gi")),
			},
		}

		pvcs, err := BuildPVCs(&crd, &vs)

		require.NoError(t, err)
		require.Len(t, pvcs, 1)

		got := pvcs[0]

		require.Equal(t, "pvc-snapshot-my-test", got.Name)
		require.Equal(t, "test", got.Namespace)

		require.Equal(t, "my-snapshot", got.Spec.DataSource.Name)
		require.Equal(t, "VolumeSnapshot", got.Spec.DataSource.Kind)
		require.Equal(t, "snapshot.storage.k8s.io", *got.Spec.DataSource.APIGroup)
		require.Equal(t, "primo", *got.Spec.StorageClassName)
		require.Equal(t, corev1.ReadWriteOncePod, got.Spec.AccessModes[0])

		require.EqualValues(t, "10Gi", got.Spec.Resources.Requests.Storage().String())

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/component":  "HostedSnapshot",
		}
		require.Equal(t, wantLabels, got.Labels)
	})

	t.Run("no storage size", func(t *testing.T) {
		for _, tt := range []struct {
			Status *snapshotv1.VolumeSnapshotStatus
		}{
			{nil},
			{&snapshotv1.VolumeSnapshotStatus{}},
		} {
			crd := cosmosv1.HostedSnapshot{}

			vs := snapshotv1.VolumeSnapshot{
				Status: tt.Status,
			}

			_, err := BuildPVCs(&crd, &vs)
			require.Error(t, err)
		}
	})
}
