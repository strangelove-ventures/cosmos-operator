package fullnode

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PVCAutoScaler struct {
	patcher StatusPatcher
	now     func() time.Time
}

func NewPVCAutoScaler(patcher StatusPatcher) *PVCAutoScaler {
	return &PVCAutoScaler{
		patcher: patcher,
		now:     time.Now,
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
		spec         = crd.Spec.SelfHealing.PVCAutoScaling
		trigger      = int(spec.UsedSpacePercentage)
		pvcCandidate = lo.MaxBy(results, func(a PVCDiskUsage, b PVCDiskUsage) bool { return a.PercentUsed > b.PercentUsed })
	)

	// Calc new size first to catch errors with the increase quantity
	newSize, err := scaler.calcNextCapacity(pvcCandidate.Capacity, spec.IncreaseQuantity)
	if err != nil {
		return false, fmt.Errorf("calc next capacity: %w", err)
	}

	if pvcCandidate.PercentUsed < trigger {
		return false, nil
	}

	if max := spec.MaxSize; !max.IsZero() {
		// If already reached max, don't patch
		if pvcCandidate.Capacity.Cmp(max) >= 0 {
			return false, nil
		}
		// Cap new size to the maximum
		if newSize.Cmp(max) >= 0 {
			newSize = max
		}
	}

	var patch cosmosv1.CosmosFullNode
	patch.TypeMeta = crd.TypeMeta
	patch.Namespace = crd.Namespace
	patch.Name = crd.Name
	patch.Status.SelfHealing.PVCAutoScaling = &cosmosv1.PVCAutoScalingStatus{
		RequestedSize: newSize,
		RequestedAt:   metav1.NewTime(scaler.now()),
	}
	return true, scaler.patcher.Patch(ctx, &patch, client.Merge)
}

func (scaler PVCAutoScaler) calcNextCapacity(currentCapacity resource.Quantity, increase string) (resource.Quantity, error) {
	var (
		merr     error
		quantity = currentCapacity
		current  = currentCapacity.Value()
	)

	// Try to calc by percentage first
	v := intstr.FromString(increase)
	percent, err := intstr.GetScaledValueFromIntOrPercent(&v, 100, false)
	if err == nil {
		addtl := math.Round(float64(current) * (float64(percent) / 100.0))
		quantity.Set(current + int64(addtl))
		return quantity, nil
	}

	multierr.AppendInto(&merr, err)

	// Then try to calc by resource quantity
	addtl, err := resource.ParseQuantity(increase)
	if err != nil {
		return quantity, multierr.Append(merr, err)
	}

	quantity.Set(current + addtl.Value())
	return quantity, nil
}
