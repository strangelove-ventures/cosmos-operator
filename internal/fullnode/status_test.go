package fullnode

import (
	"context"
	"fmt"
	"testing"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestResetStatus(t *testing.T) {
	t.Parallel()

	var crd cosmosv1.CosmosFullNode
	crd.Generation = 123
	crd.Status.StatusMessage = ptr("should not see me")
	crd.Status.Phase = "should not see me"
	ResetStatus(&crd)

	require.EqualValues(t, 123, crd.Status.ObservedGeneration)
	require.Nil(t, crd.Status.StatusMessage)
	require.Equal(t, cosmosv1.FullNodePhaseProgressing, crd.Status.Phase)
}

type mockStatusCollector struct {
	CollectFn func(ctx context.Context, controller client.ObjectKey) cosmos.StatusCollection
}

func (m mockStatusCollector) Collect(ctx context.Context, controller client.ObjectKey) cosmos.StatusCollection {
	return m.CollectFn(ctx, controller)
}

func TestSyncInfoStatus(t *testing.T) {
	t.Parallel()

	const (
		name      = "agoric"
		namespace = "default"
	)

	var crd cosmosv1.CosmosFullNode
	crd.Name = name
	crd.Namespace = namespace

	ts := time.Now()

	var collector mockStatusCollector
	collector.CollectFn = func(ctx context.Context, controller client.ObjectKey) cosmos.StatusCollection {
		require.NotNil(t, ctx)
		require.Equal(t, name, controller.Name)
		require.Equal(t, namespace, controller.Namespace)

		var notInSync cosmos.CometStatus
		notInSync.Result.SyncInfo.CatchingUp = true
		notInSync.Result.SyncInfo.LatestBlockHeight = "9999"

		var inSync cosmos.CometStatus
		inSync.Result.SyncInfo.LatestBlockHeight = "10000"

		// Create the collection and access it directly
		collection := make(cosmos.StatusCollection, 3)

		// Fill in the details for each entry
		collection[0].Pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0"}}
		collection[0].Status = notInSync
		collection[0].TS = ts

		collection[1].Pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}}
		collection[1].Status = inSync
		collection[1].TS = ts

		collection[2].Pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}}
		collection[2].Err = fmt.Errorf("some error")
		collection[2].TS = ts

		return collection
	}

	wantTS := metav1.NewTime(ts)
	want := map[string]*cosmosv1.SyncInfoPodStatus{
		"pod-0": {
			Timestamp: wantTS,
			Height:    ptr(uint64(9999)),
			InSync:    ptr(false),
		},
		"pod-1": {
			Timestamp: wantTS,
			Height:    ptr(uint64(10000)),
			InSync:    ptr(true),
		},
		"pod-2": {
			Timestamp: wantTS,
			Error:     ptr("some error"),
		},
	}

	status := SyncInfoStatus(context.Background(), &crd, collector)
	require.Equal(t, want, status)
}
