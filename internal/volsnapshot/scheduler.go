package volsnapshot

import (
	"context"
	"fmt"
	"time"

	cosmosalpha "github.com/bharvest-devops/cosmos-operator/api/v1alpha1"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Getter is a subset of client.Client.
type Getter interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
}

// Scheduler calculates schedules using crontabs and currently running VolumeSnapshots.
type Scheduler struct {
	getter Getter
	now    func() time.Time
}

func NewScheduler(getter Getter) *Scheduler {
	return &Scheduler{
		getter: getter,
		now:    time.Now,
	}
}

// CalcNext the duration until it's time to take the next snapshot.
// A zero duration without an error indicates a VolumeSnapshot should be created.
// Updates crd.status with the last VolumeSnapshot status.
func (s Scheduler) CalcNext(crd *cosmosalpha.ScheduledVolumeSnapshot) (time.Duration, error) {
	sched, err := cron.ParseStandard(crd.Spec.Schedule)
	if err != nil {
		return 0, fmt.Errorf("invalid spec.schedule: %w", err)
	}

	refDate := crd.Status.CreatedAt.Time
	if snapStatus := crd.Status.LastSnapshot; snapStatus != nil {
		refDate = snapStatus.StartedAt.Time
	}

	next := sched.Next(refDate)
	return lo.Max([]time.Duration{next.Sub(s.now()), 0}), nil
}

// IsSnapshotReady returns true if the status.LastSnapshot is ready for use and updates the crd.status.lastSnapshot.
// A non-nil error can be treated as transient.
// If VolumeSnapshot is not found, this indicates a rare case where something deleted the VolumeSnapshot before
// detecting if it's ready. In that case, this method returns that the snapshot is ready.
func (s Scheduler) IsSnapshotReady(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) (bool, error) {
	var snapshot snapshotv1.VolumeSnapshot
	snapshot.Name = crd.Status.LastSnapshot.Name
	snapshot.Namespace = crd.Namespace

	err := s.getter.Get(ctx, client.ObjectKeyFromObject(&snapshot), &snapshot)
	switch {
	case kube.IsNotFound(err):
		return true, nil
	case err != nil:
		return false, err
	}

	crd.Status.LastSnapshot.Status = snapshot.Status
	return kube.VolumeSnapshotIsReady(snapshot.Status), nil
}
