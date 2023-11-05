package fullnode

import (
	"context"
	"errors"
	"math"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusSyncer interface {
	SyncUpdate(ctx context.Context, key client.ObjectKey, update func(status *cosmosv1.FullNodeStatus)) error
}

type PVCAutoScaler struct {
	client StatusSyncer
	now    func() time.Time
}

func NewPVCAutoScaler(client StatusSyncer) *PVCAutoScaler {
	return &PVCAutoScaler{
		client: client,
		now:    time.Now,
	}
}

// SignalPVCResize patches the CosmosFullNode.status.selfHealing with the new calculated PVC size as a resource quantity.
// Assumes CosmosfullNode.spec.selfHealing.pvcAutoScaling is set or else this method may panic.
// The CosmosFullNode controller is responsible for increasing the PVC disk size.
//
// Returns true if the status was patched.
//
// Returns false and does not patch if:
// 1. The PVCs do not need resizing
// 2. The status already has >= calculated size.
// 3. The maximum size has been reached. It will patch up to the maximum size.
//
// Returns an error if patching unsuccessful.
func (scaler PVCAutoScaler) SignalPVCResize(ctx context.Context, crd *cosmosv1.CosmosFullNode, results []PVCDiskUsage) (bool, error) {
	var (
		spec    = crd.Spec.SelfHeal.PVCAutoScale
		trigger = int(spec.UsedSpacePercentage)
	)

	var joinedErr error

	status := crd.Status.SelfHealing.PVCAutoScale

	patches := make(map[string]*cosmosv1.PVCAutoScaleStatus)

	now := metav1.NewTime(scaler.now())

	for _, pvc := range results {
		if pvc.PercentUsed < trigger {
			// no need to expand
			continue
		}

		newSize, err := scaler.calcNextCapacity(pvc.Capacity, spec.IncreaseQuantity)
		if err != nil {
			joinedErr = errors.Join(joinedErr, err)
			continue
		}

		if status != nil {
			if pvcStatus, ok := status[pvc.Name]; ok && pvcStatus.RequestedSize.Value() == newSize.Value() {
				// already requested
				continue
			}
		}

		if max := spec.MaxSize; !max.IsZero() {
			if pvc.Capacity.Cmp(max) >= 0 {
				// already at max size
				continue
			}

			if newSize.Cmp(max) >= 0 {
				// Cap new size to the max size
				newSize = max
			}
		}

		patches[pvc.Name] = &cosmosv1.PVCAutoScaleStatus{
			RequestedSize: newSize,
			RequestedAt:   now,
		}
	}

	if len(patches) == 0 {
		return false, joinedErr
	}

	return true, errors.Join(joinedErr, scaler.client.SyncUpdate(ctx, client.ObjectKeyFromObject(crd), func(status *cosmosv1.FullNodeStatus) {
		if status.SelfHealing.PVCAutoScale == nil {
			status.SelfHealing.PVCAutoScale = patches
			return
		}
		for k, v := range patches {
			status.SelfHealing.PVCAutoScale[k] = v
		}
	}))
}

func (scaler PVCAutoScaler) calcNextCapacity(current resource.Quantity, increase string) (resource.Quantity, error) {
	var (
		merr     error
		quantity resource.Quantity
	)

	// Try to calc by percentage first
	v := intstr.FromString(increase)
	percent, err := intstr.GetScaledValueFromIntOrPercent(&v, 100, false)
	if err == nil {
		addtl := math.Round(float64(current.Value()) * (float64(percent) / 100.0))
		quantity = *resource.NewQuantity(current.Value()+int64(addtl), current.Format)
		return quantity, nil
	}

	merr = errors.Join(merr, err)

	// Then try to calc by resource quantity
	addtl, err := resource.ParseQuantity(increase)
	if err != nil {
		return quantity, errors.Join(merr, err)
	}

	return *resource.NewQuantity(current.Value()+addtl.Value(), current.Format), nil
}
