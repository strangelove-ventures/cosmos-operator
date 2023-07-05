package fullnode

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDriftDetection_LaggingPods(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "noble"
		crd.Namespace = "default"
		maxUnavail := &intstr.IntOrString{Type: intstr.Int, IntVal: 1}
		crd.Spec.RolloutStrategy.MaxUnavailable = maxUnavail
		crd.Spec.Replicas = 3

		var coll cosmos.StatusCollection = lo.Map(lo.Range(5), func(_, i int) cosmos.StatusItem {
			return cosmos.StatusItem{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i)}}}
		})

		coll[0].Status.Result.SyncInfo.LatestBlockHeight = "100"
		coll[1].Status.Result.SyncInfo.LatestBlockHeight = "100"
		coll[2].Status.Result.SyncInfo.LatestBlockHeight = "95"
		coll[3].Status.Result.SyncInfo.LatestBlockHeight = "90"
		coll[4].Status.Result.SyncInfo.CatchingUp = true

		collector := mockStatusCollector{CollectFn: func(ctx context.Context, controller client.ObjectKey) cosmos.StatusCollection {
			require.NotNil(t, ctx)
			require.Equal(t, client.ObjectKey{Namespace: "default", Name: "noble"}, controller)
			return coll
		}}

		detector := NewDriftDetection(collector)

		for _, tt := range []struct {
			Threshold uint32
			Available int
			WantPods  []string
		}{
			{10, 1, []string{"pod-3"}},
			{5, 10, []string{"pod-2", "pod-3"}},
			{3, 1, []string{"pod-2"}},
			{1, 0, []string{}},
		} {
			crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{}
			crd.Spec.SelfHeal.HeightDriftMitigation = &cosmosv1.HeightDriftMitigationSpec{
				Threshold: tt.Threshold,
			}

			detector.available = func(pods []*corev1.Pod, minReady time.Duration, now time.Time) []*corev1.Pod {
				require.GreaterOrEqual(t, len(pods), len(tt.WantPods))
				require.WithinDuration(t, time.Now(), now, 5*time.Second)
				require.Equal(t, 5*time.Second, minReady)
				return coll.SyncedPods()
			}

			detector.computeRollout = func(unavail *intstr.IntOrString, desired, ready int) int {
				require.Equal(t, maxUnavail, unavail, tt)
				require.EqualValues(t, crd.Spec.Replicas, desired, tt)
				require.NotZero(t, ready, tt)
				require.Equal(t, len(coll.SyncedPods()), ready, tt)
				return tt.Available
			}

			got := detector.LaggingPods(context.Background(), &crd)
			gotPods := lo.Map(got, func(pod *corev1.Pod, _ int) string { return pod.Name })

			require.Equal(t, tt.WantPods, gotPods, tt)
		}
	})

	t.Run("no pods or replicas", func(t *testing.T) {
		collector := mockStatusCollector{CollectFn: func(ctx context.Context, controller client.ObjectKey) cosmos.StatusCollection {
			return nil
		}}
		detector := NewDriftDetection(collector)

		var crd cosmosv1.CosmosFullNode
		crd.Spec.SelfHeal = &cosmosv1.SelfHealSpec{}
		crd.Spec.SelfHeal.HeightDriftMitigation = &cosmosv1.HeightDriftMitigationSpec{
			Threshold: 25,
		}

		got := detector.LaggingPods(context.Background(), &crd)
		require.Empty(t, got)

		crd.Spec.Replicas = 3
		got = detector.LaggingPods(context.Background(), &crd)
		require.Empty(t, got)
	})
}
