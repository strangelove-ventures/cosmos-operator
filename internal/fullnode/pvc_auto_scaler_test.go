package fullnode

import (
	"context"
	"errors"
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
			// Weird user input cases
			{"1", zeroQuant, *resource.NewQuantity(capacity.Value()+1, resource.BinarySI)},
		} {
			var crd cosmosv1.CosmosFullNode
			crd.APIVersion = "v1"
			crd.Name = name
			crd.Namespace = namespace
			crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{
				PVCAutoScale: &cosmosv1.PVCAutoScaleSpec{
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
				require.Equal(t, want.ObjectMeta, got.ObjectMeta, tt)
				require.Equal(t, want.TypeMeta, got.TypeMeta, tt)
				require.Empty(t, got.Spec, tt) // Asserts we just patch the status

				gotStatus := got.Status.SelfHealing.PVCAutoScale
				require.Equal(t, stubNow, gotStatus.RequestedAt.Time, tt)
				require.Truef(t, tt.Want.Equal(gotStatus.RequestedSize), "%s:\nwant %+v\ngot  %+v", tt, tt.Want, gotStatus.RequestedSize)

				require.Equal(t, client.Apply, patch)

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
		crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{
			PVCAutoScale: &cosmosv1.PVCAutoScaleSpec{
				UsedSpacePercentage: usedSpacePercentage,
				IncreaseQuantity:    "300%",
				MaxSize:             maxSize,
			},
		}

		var patchCalled bool
		patcher := mockPatcher(func(_ context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			got := obj.(*cosmosv1.CosmosFullNode)
			gotStatus := got.Status.SelfHealing.PVCAutoScale
			require.Equal(t, maxSize.Value(), gotStatus.RequestedSize.Value())
			require.Equal(t, maxSize.Format, gotStatus.RequestedSize.Format)

			require.Equal(t, client.Apply, patch)

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

	t.Run("capacity at or above max", func(t *testing.T) {
		for _, tt := range []struct {
			Max, Capacity resource.Quantity
		}{
			{resource.MustParse("5Ti"), resource.MustParse("5Ti")}, // the same
			{resource.MustParse("1G"), resource.MustParse("2G")},   // greater
		} {
			const usedSpacePercentage = 60

			var crd cosmosv1.CosmosFullNode
			crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{
				PVCAutoScale: &cosmosv1.PVCAutoScaleSpec{
					UsedSpacePercentage: usedSpacePercentage,
					IncreaseQuantity:    "10Gi",
					MaxSize:             tt.Max,
				},
			}

			scaler := NewPVCAutoScaler(panicPatcher)
			usage := []PVCDiskUsage{
				{PercentUsed: 80, Capacity: tt.Capacity},
			}
			got, err := scaler.SignalPVCResize(ctx, &crd, usage)

			require.NoError(t, err, tt)
			require.False(t, got, tt)
		}
	})

	t.Run("no patch needed", func(t *testing.T) {
		for _, tt := range []struct {
			DiskUsage []PVCDiskUsage
		}{
			{nil}, // tests zero state
			{[]PVCDiskUsage{
				{PercentUsed: 79},
				{PercentUsed: 1},
				{PercentUsed: 10},
			}},
		} {
			var crd cosmosv1.CosmosFullNode
			crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{
				PVCAutoScale: &cosmosv1.PVCAutoScaleSpec{
					UsedSpacePercentage: 80,
					IncreaseQuantity:    "10Gi",
				},
			}

			scaler := NewPVCAutoScaler(panicPatcher)
			got, err := scaler.SignalPVCResize(ctx, &crd, lo.Shuffle(tt.DiskUsage))

			require.NoError(t, err)
			require.False(t, got)
		}
	})

	t.Run("patch already signaled", func(t *testing.T) {
		const usedSpacePercentage = 90

		var crd cosmosv1.CosmosFullNode
		crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{
			PVCAutoScale: &cosmosv1.PVCAutoScaleSpec{
				UsedSpacePercentage: usedSpacePercentage,
				IncreaseQuantity:    "10Gi",
			},
		}
		crd.Status.SelfHealing.PVCAutoScale = &cosmosv1.PVCAutoScaleStatus{
			RequestedSize: resource.MustParse("100Gi"),
		}

		scaler := NewPVCAutoScaler(panicPatcher)
		usage := []PVCDiskUsage{
			{PercentUsed: usedSpacePercentage, Capacity: resource.MustParse("90Gi")},
		}
		got, err := scaler.SignalPVCResize(ctx, &crd, usage)

		require.NoError(t, err)
		require.False(t, got)
	})

	t.Run("invalid increase quantity", func(t *testing.T) {
		const usedSpacePercentage = 80

		for _, tt := range []struct {
			Increase string
		}{
			{""}, // CRD validation should prevent this
			{"wut"},
		} {
			var crd cosmosv1.CosmosFullNode
			crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{
				PVCAutoScale: &cosmosv1.PVCAutoScaleSpec{
					UsedSpacePercentage: usedSpacePercentage,
					IncreaseQuantity:    tt.Increase,
				},
			}

			scaler := NewPVCAutoScaler(panicPatcher)
			usage := []PVCDiskUsage{
				{PercentUsed: usedSpacePercentage},
			}
			_, err := scaler.SignalPVCResize(ctx, &crd, lo.Shuffle(usage))

			require.Error(t, err)
			require.Contains(t, err.Error(), "increaseQuantity must be a percentage string (e.g. 10%) or a storage quantity (e.g. 100Gi):")
		}
	})

	t.Run("patch error", func(t *testing.T) {
		const usedSpacePercentage = 50

		var crd cosmosv1.CosmosFullNode
		crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{
			PVCAutoScale: &cosmosv1.PVCAutoScaleSpec{
				UsedSpacePercentage: usedSpacePercentage,
				IncreaseQuantity:    "10%",
			},
		}

		scaler := NewPVCAutoScaler(mockPatcher(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			return errors.New("boom")
		}))
		usage := []PVCDiskUsage{
			{PercentUsed: usedSpacePercentage},
		}
		_, err := scaler.SignalPVCResize(ctx, &crd, lo.Shuffle(usage))

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}
