## StatefulJob

Status: v1alpha1

**Warning: May have backwards breaking changes!**

A StatefulJob is a means to process persistent data from a recent [VolumeSnapshot](https://kubernetes.io/docs/concepts/storage/volume-snapshots/).
It periodically creates a job and PVC using the most recent VolumeSnapshot as its data source. It mounts the PVC as volume "snapshot" into the job's pod.
The user must configure container volume mounts.
It's similar to a CronJob but does not offer advanced scheduling via a crontab.

Strangelove uses it to compress and upload snapshots of chain data.

[Example yaml](../config/samples/cosmos_v1alpha1_statefuljob.yaml)
