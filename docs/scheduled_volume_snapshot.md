## ScheduledVolumeSnapshot

Status: v1alpha1

**Warning: May have backwards breaking changes!**

ScheduledVolumeSnapshot allows you to periodically backup your data.

It creates [VolumeSnapshots]([VolumeSnapshot](https://kubernetes.io/docs/concepts/storage/volume-snapshots/))
from a healthy CosmosFullNode PVC on a recurring schedule described by a [crontab](https://en.wikipedia.org/wiki/Cron). The controller
chooses a candidate pod/pvc combo from a source CosmosFullNode. This allows you to create reliable, scheduled backups
of blockchain state.

**Warning: Backups may include private keys and secrets.** For validators, we strongly recommend using [Horcrux](https://github.com/strangelove-ventures/horcrux),
[TMKMS](https://github.com/iqlusioninc/tmkms), or another CometBFT remote signer.

To minimize data corruption, the operator temporarily deletes the CosmosFullNode pod writing to the PVC while taking the snapshot. Deleting the pod allows the process to
exit gracefully and prevents writes to the disk. Once the snapshot is complete, the operator re-creates the pod. Therefore, use of this CRD may affect
availability of the source CosmosFullNode. At least 2 CosmosFullNode replicas is necessary to prevent downtime; 3
replicas recommended. In the future, this behavior may be configurable.

Limitations:
- The CosmosFullNode and ScheduledVolumeSnapshot must be in the same namespace.

[Example yaml](../config/samples/cosmos_v1alpha1_scheduledvolumesnapshot.yaml)
