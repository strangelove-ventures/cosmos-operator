package fullnode

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockPatcher func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error

func (fn mockPatcher) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if ctx == nil {
		panic("nil context")
	}
	if len(opts) > 0 {
		panic("unexpected opts")
	}
	return fn(ctx, obj, patch, opts...)
}

func TestPVCAutoScaler_SignalPVCResize(t *testing.T) {
	t.Parallel()
	rand.Seed(time.Now().UnixNano())

	ctx := context.Background()

	panicPatcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
		panic("patch should not be called")
	})

	t.Run("happy path", func(t *testing.T) {
		var (
			capacity  = resource.MustParse("100Gi")
			stubNow   = time.Now()
			zeroQuant resource.Quantity
		)
		const (
			usedSpacePercentage = 80
			name                = "auto-scale-test"
			namespace           = "strangelove"
		)

		for _, tt := range []struct {
			Increase string
			Max      resource.Quantity
			Want     resource.Quantity
		}{
			{"20Gi", resource.MustParse("500Gi"), resource.MustParse("120Gi")},
			{"10%", zeroQuant, resource.MustParse("110Gi")},
			{"0.5Gi", zeroQuant, resource.MustParse("100.5Gi")},
			{"200%", zeroQuant, resource.MustParse("300Gi")},
		} {
			var crd cosmosv1.CosmosFullNode
			crd.APIVersion = "v1"
			crd.Name = name
			crd.Namespace = namespace
			crd.Spec.SelfHealing = &cosmosv1.SelfHealingSpec{
				PVCAutoScaling: &cosmosv1.PVCAutoScalingSpec{
					UsedSpacePercentage: usedSpacePercentage,
					IncreaseQuantity:    tt.Increase,
					MaxSize:             tt.Max,
				},
			}

			var patchCalled bool
			patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				var want cosmosv1.CosmosFullNode
				want.Name = name
				want.Namespace = namespace
				want.TypeMeta = crd.TypeMeta

				got := obj.(*cosmosv1.CosmosFullNode)
				require.Equal(t, want.ObjectMeta, got.ObjectMeta)
				require.Equal(t, want.TypeMeta, got.TypeMeta)
				require.Empty(t, got.Spec) // Asserts we just patch the status

				gotStatus := got.Status.SelfHealing.PVCAutoScaling
				require.Equal(t, stubNow, gotStatus.RequestedAt.Time)
				require.Equal(t, tt.Want.Value(), gotStatus.RequestedSize.Value())
				require.Equal(t, tt.Want.Format, gotStatus.RequestedSize.Format)

				require.Equal(t, client.Merge, patch)

				patchCalled = true
				return nil
			})
			scaler := NewPVCAutoScaler(patcher)
			scaler.now = func() time.Time {
				return stubNow
			}

			trigger := 80 + rand.Intn(20)
			usage := []PVCDiskUsage{
				{PercentUsed: trigger, Capacity: capacity},
				{PercentUsed: 10},
				{PercentUsed: 79},
			}
			got, err := scaler.SignalPVCResize(ctx, &crd, lo.Shuffle(usage))

			require.NoError(t, err, tt)
			require.True(t, got, tt)
			require.True(t, patchCalled, tt)
		}
	})

	t.Run("does not exceed max", func(t *testing.T) {
		var (
			capacity = resource.MustParse("100Ti")
			maxSize  = resource.MustParse("200Ti")
		)
		const usedSpacePercentage = 80

		var crd cosmosv1.CosmosFullNode
		crd.Spec.SelfHealing = &cosmosv1.SelfHealingSpec{
			PVCAutoScaling: &cosmosv1.PVCAutoScalingSpec{
				UsedSpacePercentage: usedSpacePercentage,
				IncreaseQuantity:    "300%",
				MaxSize:             maxSize,
			},
		}

		var patchCalled bool
		patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			got := obj.(*cosmosv1.CosmosFullNode)
			gotStatus := got.Status.SelfHealing.PVCAutoScaling
			require.Equal(t, maxSize.Value(), gotStatus.RequestedSize.Value())
			require.Equal(t, maxSize.Format, gotStatus.RequestedSize.Format)

			require.Equal(t, client.Merge, patch)

			patchCalled = true
			return nil
		})
		scaler := NewPVCAutoScaler(patcher)

		usage := []PVCDiskUsage{
			{PercentUsed: 80, Capacity: capacity},
		}
		got, err := scaler.SignalPVCResize(ctx, &crd, lo.Shuffle(usage))

		require.NoError(t, err)
		require.True(t, got)
		require.True(t, patchCalled)
	})

	t.Run("capacity at max", func(t *testing.T) {
		var maxSize = resource.MustParse("5Ti")
		const usedSpacePercentage = 80

		var crd cosmosv1.CosmosFullNode
		crd.Spec.SelfHealing = &cosmosv1.SelfHealingSpec{
			PVCAutoScaling: &cosmosv1.PVCAutoScalingSpec{
				UsedSpacePercentage: usedSpacePercentage,
				IncreaseQuantity:    "10Gi",
				MaxSize:             maxSize,
			},
		}

		scaler := NewPVCAutoScaler(panicPatcher)
		usage := []PVCDiskUsage{
			{PercentUsed: 80, Capacity: maxSize},
		}
		got, err := scaler.SignalPVCResize(ctx, &crd, lo.Shuffle(usage))

		require.NoError(t, err)
		require.False(t, got)
	})

	t.Run("no patch needed", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Spec.SelfHealing = &cosmosv1.SelfHealingSpec{
			PVCAutoScaling: &cosmosv1.PVCAutoScalingSpec{
				UsedSpacePercentage: 80,
				IncreaseQuantity:    "10Gi",
			},
		}

		scaler := NewPVCAutoScaler(panicPatcher)
		usage := []PVCDiskUsage{
			{PercentUsed: 79},
			{PercentUsed: 1},
			{PercentUsed: 10},
		}
		got, err := scaler.SignalPVCResize(ctx, &crd, lo.Shuffle(usage))

		require.NoError(t, err)
		require.False(t, got)
	})

	t.Run("no disk usage results", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Spec.SelfHealing = &cosmosv1.SelfHealingSpec{
			PVCAutoScaling: &cosmosv1.PVCAutoScalingSpec{
				UsedSpacePercentage: 80,
				IncreaseQuantity:    "10Gi",
			},
		}

		scaler := NewPVCAutoScaler(panicPatcher)
		got, err := scaler.SignalPVCResize(ctx, &crd, nil)
		require.NoError(t, err)
		require.False(t, got)
	})

	t.Run("invalid increase quantity", func(t *testing.T) {

	})

	t.Run("patch error", func(t *testing.T) {

	})
}
