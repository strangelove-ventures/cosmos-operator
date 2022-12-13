package volsnapshot

import (
	"context"
	"fmt"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
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
func (s Scheduler) CalcNext(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) (time.Duration, kube.ReconcileError) {
	sched, err := cron.ParseStandard(crd.Spec.Schedule)
	if err != nil {
		return 0, kube.UnrecoverableError(fmt.Errorf("invalid spec.schedule: %w", err))
	}

	var (
		refDate = s.refDate(crd)
		next    = sched.Next(refDate)
		dur     = lo.Max([]time.Duration{next.Sub(s.now()), 0})
	)

	isReady, err := s.snapshotReady(ctx, crd)
	switch {
	case kube.IsNotFound(err):
		// Hopefully rare case. Means something or someone deleted the VolumeSnapshot before controller could detect
		// it was ready for use. Assume snapshot completed if not found.
		return dur, nil
	case err != nil:
		return 0, kube.TransientError(err)
	}

	if !isReady {
		// If not ready for use, indicate a requeue in the near future to check again.
		return 10 * time.Second, nil
	}

	return dur, nil
}

func (s Scheduler) refDate(crd *cosmosalpha.ScheduledVolumeSnapshot) time.Time {
	if snapStatus := crd.Status.LastSnapshot; snapStatus != nil {
		return snapStatus.StartedAt.Time
	}
	return crd.Status.CreatedAt.Time
}

func (s Scheduler) snapshotReady(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) (bool, error) {
	if crd.Status.LastSnapshot == nil {
		return true, nil
	}

	if existing := crd.Status.LastSnapshot.Status; existing != nil {
		// Prevent calling API if we already know snapshot is ready.
		if kube.VolumeSnapshotIsReady(existing) {
			return true, nil
		}
	}

	var snapshot snapshotv1.VolumeSnapshot
	snapshot.Name = crd.Status.LastSnapshot.Name
	snapshot.Namespace = crd.Namespace

	if err := s.getter.Get(ctx, client.ObjectKeyFromObject(&snapshot), &snapshot); err != nil {
		return false, err
	}
	crd.Status.LastSnapshot.Status = snapshot.Status
	return kube.VolumeSnapshotIsReady(snapshot.Status), nil
}
