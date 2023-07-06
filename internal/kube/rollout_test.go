package kube

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestComputeRollout(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		for _, tt := range []struct {
			Unavail        intstr.IntOrString
			Desired, Ready int
			Want           int
		}{
			// All ready
			{intstr.FromInt(1), 1, 1, 1},
			{intstr.FromInt(1), 10, 10, 1},
			{intstr.FromInt(5), 10, 10, 5},

			// None ready
			{intstr.FromInt(5), 10, 0, 0},
			{intstr.FromString("44%"), 10, 0, 0},

			// Partial ready
			{intstr.FromString("50%"), 3, 3, 1},

			{intstr.FromInt(3), 9, 3, 0},
			{intstr.FromInt(3), 9, 5, 0},
			{intstr.FromInt(3), 9, 7, 1},
			{intstr.FromInt(3), 9, 8, 2},

			{intstr.FromString("35%"), 9, 4, 0},
			{intstr.FromString("35%"), 9, 6, 0},
			{intstr.FromString("35%"), 9, 7, 1},
			{intstr.FromString("35%"), 9, 8, 2},

			// Rounding down
			{intstr.FromString("33%"), 9, 8, 1},

			// Aggressive
			{intstr.FromInt(10), 10, 10, 10},
			{intstr.FromInt(20), 10, 10, 10},
			{intstr.FromInt(20), 10, 0, 10},
			{intstr.FromString("100%"), 10, 10, 10},
			{intstr.FromString("200%"), 10, 10, 10},
			{intstr.FromString("200%"), 10, 0, 10},

			// Zero max unavailable
			{intstr.FromInt(0), 100, 100, 1},
			{intstr.FromInt(0), 100, 99, 0},
			{intstr.FromString("0%"), 100, 100, 1},
			{intstr.FromString("0%"), 100, 99, 0},

			// Zero state
			{intstr.FromInt(10), 0, 0, 0},
			{intstr.FromInt(10), 0, 10, 0},
		} {
			got := ComputeRollout(&tt.Unavail, tt.Desired, tt.Ready)

			require.Equal(t, tt.Want, got, tt)
		}
	})

	t.Run("defaults to 25%", func(t *testing.T) {
		require.Equal(t, 1, ComputeRollout(nil, 4, 4))
		require.Equal(t, 0, ComputeRollout(nil, 4, 3))
		require.Equal(t, 25, ComputeRollout(nil, 100, 100))
		require.Equal(t, 1, ComputeRollout(nil, 10, 9))
		require.Equal(t, 0, ComputeRollout(nil, 10, 8))
	})
}

func FuzzComputeRollout(f *testing.F) {
	f.Add(uint(1), uint(2), uint(1))

	f.Fuzz(func(t *testing.T, maxUnavail, desired, ready uint) {
		unavail := intstr.FromInt(int(maxUnavail))
		if desired == 0 {
			desired = 1
		}
		got := ComputeRollout(&unavail, int(desired), int(ready))

		msg := fmt.Sprintf("got: %v, seeds: %v %v %v", got, maxUnavail, desired, ready)
		require.True(t, got <= int(desired), msg)
	})
}
