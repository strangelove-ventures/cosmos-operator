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
		spec         = crd.Spec.SelfHeal.PVCAutoScale
		trigger      = int(spec.UsedSpacePercentage)
		pvcCandidate = lo.MaxBy(results, func(a PVCDiskUsage, b PVCDiskUsage) bool { return a.PercentUsed > b.PercentUsed })
	)

	// Calc new size first to catch errors with the increase quantity
	newSize, err := scaler.calcNextCapacity(pvcCandidate.Capacity, spec.IncreaseQuantity)
	if err != nil {
		return false, fmt.Errorf("increaseQuantity must be a percentage string (e.g. 10%%) or a storage quantity (e.g. 100Gi): %w", err)
	}

	// Prevent patching if PVC size not at threshold
	if pvcCandidate.PercentUsed < trigger {
		return false, nil
	}

	// Prevent continuous reconcile loops
	if status := crd.Status.SelfHealing.PVCAutoScale; status != nil {
		if status.RequestedSize.Value() == newSize.Value() {
			return false, nil
		}
	}

	// Handle max size
	if max := spec.MaxSize; !max.IsZero() {
		// If already reached max size, don't patch
		if pvcCandidate.Capacity.Cmp(max) >= 0 {
			return false, nil
		}
		// Cap new size to the max size
		if newSize.Cmp(max) >= 0 {
			newSize = max
		}
	}

	// Patch object status which will signal the CosmosFullNode controller to increase PVC size.
	var patch cosmosv1.CosmosFullNode
	patch.TypeMeta = crd.TypeMeta
	patch.Namespace = crd.Namespace
	patch.Name = crd.Name
	return true, scaler.client.SyncUpdate(ctx, client.ObjectKeyFromObject(&patch), func(status *cosmosv1.FullNodeStatus) {
		status.SelfHealing.PVCAutoScale = &cosmosv1.PVCAutoScaleStatus{
			RequestedSize: newSize,
			RequestedAt:   metav1.NewTime(scaler.now()),
		}
	})
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

	multierr.AppendInto(&merr, err)

	// Then try to calc by resource quantity
	addtl, err := resource.ParseQuantity(increase)
	if err != nil {
		return quantity, multierr.Append(merr, err)
	}

	return *resource.NewQuantity(current.Value()+addtl.Value(), current.Format), nil
}
