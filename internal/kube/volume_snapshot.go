package kube

import snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

// VolumeSnapshotIsReady returns true if the snapshot is ready to use.
func VolumeSnapshotIsReady(status *snapshotv1.VolumeSnapshotStatus) bool {
	if status == nil {
		return false
	}
	if status.ReadyToUse == nil {
		return false
	}
	return *status.ReadyToUse
}
