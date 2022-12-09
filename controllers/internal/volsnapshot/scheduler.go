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

	refDate := crd.Status.CreatedAt.Time
	//if crd.Status.LastSnapshot != nil {
	//	refDate = ts
	//}

	next := sched.Next(refDate)
	return lo.Max([]time.Duration{next.Sub(s.now()), 0}), nil
}

func (s Scheduler) refDateFromLastSnapshot(ctx context.Context, sched cron.Schedule, crd *cosmosalpha.ScheduledVolumeSnapshot) (time.Time, error) {
	var snapshot snapshotv1.VolumeSnapshot
	snapshot.Name = crd.Status.LastSnapshot.Name
	snapshot.Namespace = crd.Namespace

	refDate := crd.Status.LastSnapshot.StartedAt.Time

	err := s.getter.Get(ctx, client.ObjectKeyFromObject(&snapshot), &snapshot)
	switch {
	case kube.IsNotFound(err):
		return sched.Next(refDate), nil
	case err != nil:
		return time.Time{}, err
	}

	if kube.VolumeSnapshotIsReady(snapshot.Status) {
		return crd.Status.
	}
	return 10*time.Second, nil
}
