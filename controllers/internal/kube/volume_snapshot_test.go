package kube

import (
	"testing"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/require"
)

func TestVolumeSnapshotIsReady(t *testing.T) {
	t.Parallel()

	var (
		isReady  = true
		notReady bool
	)

	for _, tt := range []struct {
		Status *snapshotv1.VolumeSnapshotStatus
		Want   bool
	}{
		{nil, false},
		{new(snapshotv1.VolumeSnapshotStatus), false},
		{&snapshotv1.VolumeSnapshotStatus{ReadyToUse: &notReady}, false},

		{&snapshotv1.VolumeSnapshotStatus{ReadyToUse: &isReady}, true},
	} {
		require.Equal(t, tt.Want, VolumeSnapshotIsReady(tt.Status), tt)
	}
}
