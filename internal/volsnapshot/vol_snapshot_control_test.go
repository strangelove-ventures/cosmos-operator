package volsnapshot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	cosmosalpha "github.com/bharvest-devops/cosmos-operator/api/v1alpha1"
	"github.com/bharvest-devops/cosmos-operator/internal/fullnode"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockPodClient struct {
	GotListOpts []client.ListOption
	Items       []corev1.Pod
	ListErr     error

	GotCreateObj client.Object
	CreateErr    error
}

func (m *mockPodClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if ctx == nil {
		panic("nil context")
	}
	if len(opts) > 0 {
		panic(fmt.Errorf("expected 0 opts, got %d", len(opts)))
	}
	m.GotCreateObj = obj
	return m.CreateErr
}

func (m *mockPodClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.GotListOpts = opts
	list.(*corev1.PodList).Items = m.Items
	return m.ListErr
}

func (m *mockPodClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	panic("delete should not be called")
}

type mockPodFilter struct {
	SyncedPodsFn func(ctx context.Context, controller client.ObjectKey) []*corev1.Pod
}

func (fn mockPodFilter) SyncedPods(ctx context.Context, controller client.ObjectKey) []*corev1.Pod {
	if ctx == nil {
		panic("nil context")
	}
	if fn.SyncedPodsFn == nil {
		panic("SyncedPods not implemented")
	}
	return fn.SyncedPodsFn(ctx, controller)
}

var (
	panicFilter mockPodFilter
	nopLogger   = logr.Discard()
)

func TestVolumeSnapshotControl_FindCandidate(t *testing.T) {
	t.Parallel()

	var (
		ctx            = context.Background()
		readyCondition = corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue}
	)

	const (
		fullNodeName = "cosmoshub"
		namespace    = "strangelove"
	)

	var crd cosmosalpha.ScheduledVolumeSnapshot
	crd.Name = "test"
	crd.Namespace = namespace
	crd.Spec.FullNodeRef.Name = fullNodeName

	t.Run("happy path", func(t *testing.T) {
		pods := make([]corev1.Pod, 3)
		for i := range pods {
			pods[i].Status.Conditions = []corev1.PodCondition{readyCondition}
		}
		var mClient mockPodClient
		mClient.Items = pods

		var fullnodeCRD cosmosv1.CosmosFullNode
		fullnodeCRD.Name = fullNodeName
		appConfig := cosmosv1.SDKAppConfig{}
		fullnodeCRD.Spec.ChainSpec.CosmosSDK = &appConfig
		// Purposefully using PodBuilder to cross-test any breaking changes in PodBuilder which affects
		// finding the PVC name.
		candidate, err := fullnode.NewPodBuilder(&fullnodeCRD).WithOrdinal(1).Build()
		require.NoError(t, err)

		control := NewVolumeSnapshotControl(&mClient, mockPodFilter{
			SyncedPodsFn: func(ctx context.Context, controller client.ObjectKey) []*corev1.Pod {
				require.Equal(t, namespace, controller.Namespace)
				require.Equal(t, fullNodeName, controller.Name)
				return []*corev1.Pod{candidate, new(corev1.Pod), new(corev1.Pod)}
			},
		})

		got, err := control.FindCandidate(ctx, &crd)
		require.NoError(t, err)

		require.Equal(t, "cosmoshub-1", got.PodName)
		require.Equal(t, "pvc-cosmoshub-1", got.PVCName)
		require.NotEmpty(t, got.PodLabels)
		require.Equal(t, candidate.Labels, got.PodLabels)
	})

	t.Run("happy path with index", func(t *testing.T) {
		pods := make([]corev1.Pod, 3)
		for i := range pods {
			pods[i].Status.Conditions = []corev1.PodCondition{readyCondition}
		}
		var mClient mockPodClient
		mClient.Items = pods

		var fullnodeCRD cosmosv1.CosmosFullNode
		fullnodeCRD.Name = fullNodeName
		appConfig := cosmosv1.SDKAppConfig{}
		fullnodeCRD.Spec.ChainSpec.CosmosSDK = &appConfig
		// Purposefully using PodBuilder to cross-test any breaking changes in PodBuilder which affects
		// finding the PVC name.
		candidate, err := fullnode.NewPodBuilder(&fullnodeCRD).WithOrdinal(1).Build()
		require.NoError(t, err)

		candidate.Annotations["app.kubernetes.io/ordinal"] = "1"

		control := NewVolumeSnapshotControl(&mClient, mockPodFilter{
			SyncedPodsFn: func(ctx context.Context, controller client.ObjectKey) []*corev1.Pod {
				require.Equal(t, namespace, controller.Namespace)
				require.Equal(t, fullNodeName, controller.Name)
				return []*corev1.Pod{candidate, new(corev1.Pod), new(corev1.Pod)}
			},
		})

		indexCRD := crd.DeepCopy()
		index := int32(1)
		indexCRD.Spec.FullNodeRef.Ordinal = &index

		got, err := control.FindCandidate(ctx, indexCRD)
		require.NoError(t, err)

		require.Equal(t, "cosmoshub-1", got.PodName)
		require.Equal(t, "pvc-cosmoshub-1", got.PVCName)
		require.NotEmpty(t, got.PodLabels)
		require.Equal(t, candidate.Labels, got.PodLabels)
	})

	t.Run("index not available", func(t *testing.T) {
		pods := make([]corev1.Pod, 3)
		for i := range pods {
			pods[i].Status.Conditions = []corev1.PodCondition{readyCondition}
		}
		var mClient mockPodClient
		mClient.Items = pods

		var fullnodeCRD cosmosv1.CosmosFullNode
		fullnodeCRD.Name = fullNodeName
		appConfig := cosmosv1.SDKAppConfig{}
		fullnodeCRD.Spec.ChainSpec.CosmosSDK = &appConfig
		// Purposefully using PodBuilder to cross-test any breaking changes in PodBuilder which affects
		// finding the PVC name.
		candidate, err := fullnode.NewPodBuilder(&fullnodeCRD).WithOrdinal(1).Build()
		require.NoError(t, err)

		control := NewVolumeSnapshotControl(&mClient, mockPodFilter{
			SyncedPodsFn: func(ctx context.Context, controller client.ObjectKey) []*corev1.Pod {
				require.Equal(t, namespace, controller.Namespace)
				require.Equal(t, fullNodeName, controller.Name)
				return []*corev1.Pod{candidate, new(corev1.Pod), new(corev1.Pod)}
			},
		})

		indexCRD := crd.DeepCopy()
		index := int32(2)
		indexCRD.Spec.FullNodeRef.Ordinal = &index

		_, err = control.FindCandidate(ctx, indexCRD)
		require.ErrorContains(t, err, "in-sync pod with index 2 not found")
	})

	t.Run("custom min available", func(t *testing.T) {
		var pod corev1.Pod
		pod.Name = "found-me"
		pod.Status.Conditions = []corev1.PodCondition{readyCondition}
		var mClient mockPodClient
		mClient.Items = []corev1.Pod{pod}

		control := NewVolumeSnapshotControl(&mClient, mockPodFilter{
			SyncedPodsFn: func(context.Context, client.ObjectKey) []*corev1.Pod {
				return []*corev1.Pod{&pod}
			},
		})

		availCRD := crd.DeepCopy()
		availCRD.Spec.MinAvailable = 1

		got, err := control.FindCandidate(ctx, availCRD)

		require.NoError(t, err)
		require.Equal(t, "found-me", got.PodName)
	})

	t.Run("not enough ready pods", func(t *testing.T) {
		var readyPod corev1.Pod
		readyPod.Status.Conditions = []corev1.PodCondition{readyCondition}

		for i, tt := range []struct {
			Pods    []corev1.Pod
			WantErr string
		}{
			{nil, "2 or more pods must be in-sync to prevent downtime, found 0 in-sync"},
			{make([]corev1.Pod, 0), "2 or more pods must be in-sync to prevent downtime, found 0 in-sync"},
			{make([]corev1.Pod, 1), "2 or more pods must be in-sync to prevent downtime, found 1 in-sync"}, // no pods in-sync
		} {
			var mClient mockPodClient
			control := NewVolumeSnapshotControl(&mClient, mockPodFilter{
				SyncedPodsFn: func(context.Context, client.ObjectKey) []*corev1.Pod {
					return ptrSlice(tt.Pods)
				},
			})

			_, err := control.FindCandidate(ctx, &crd)

			require.Errorf(t, err, "test case %d", i)
			require.EqualErrorf(t, err, tt.WantErr, "test case %d", i)
		}
	})
}

func TestVolumeSnapshotControl_CreateSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		var mClient mockPodClient
		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		// Use time.Local to ensure we format with UTC.
		now := time.Date(2022, time.September, 1, 2, 3, 0, 0, time.UTC)
		control.now = func() time.Time {
			return now
		}

		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Name = "my-snapshot"
		crd.Namespace = "strangelove"
		crd.Spec.VolumeSnapshotClassName = "my-snap-class"

		labels := map[string]string{
			"test":               "labels",
			kube.ControllerLabel: "should not see me",
			kube.ComponentLabel:  "should not see me",
		}
		candidate := Candidate{
			PodLabels: labels,
			PodName:   "chain-1",
			PVCName:   "pvc-chain-1",
		}
		err := control.CreateSnapshot(ctx, &crd, candidate)

		require.NoError(t, err)
		require.NotNil(t, mClient.GotCreateObj)

		got := mClient.GotCreateObj.(*snapshotv1.VolumeSnapshot)
		require.Equal(t, "strangelove", got.Namespace)
		const wantName = "my-snapshot-202209010203"
		require.Equal(t, wantName, got.Name)

		require.Equal(t, "my-snap-class", *got.Spec.VolumeSnapshotClassName)
		require.Equal(t, "pvc-chain-1", *got.Spec.Source.PersistentVolumeClaimName)
		require.Nil(t, got.Spec.Source.VolumeSnapshotContentName)

		wantLabels := map[string]string{
			"test":                   "labels",
			kube.ControllerLabel:     "cosmos-operator",
			kube.ComponentLabel:      "ScheduledVolumeSnapshot",
			"cosmos.bharvest/source": "my-snapshot",
		}
		require.Equal(t, wantLabels, got.Labels)

		wantStatus := &cosmosalpha.VolumeSnapshotStatus{
			Name:      wantName,
			StartedAt: metav1.NewTime(now),
		}
		require.Equal(t, wantStatus, crd.Status.LastSnapshot)
	})

	t.Run("nil pod labels", func(t *testing.T) {
		var mClient mockPodClient
		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Name = "cosmoshub"

		err := control.CreateSnapshot(ctx, &crd, Candidate{})

		require.NoError(t, err)
		require.NotNil(t, mClient.GotCreateObj)

		got := mClient.GotCreateObj.(*snapshotv1.VolumeSnapshot)

		wantLabels := map[string]string{
			kube.ControllerLabel: "cosmos-operator",
			kube.ComponentLabel:  "ScheduledVolumeSnapshot",
			cosmosSourceLabel:    "cosmoshub",
		}
		require.Equal(t, wantLabels, got.Labels)
	})

	t.Run("create error", func(t *testing.T) {
		var mClient mockPodClient
		mClient.CreateErr = errors.New("boom")
		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		var crd cosmosalpha.ScheduledVolumeSnapshot
		err := control.CreateSnapshot(ctx, &crd, Candidate{})

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}

type mockVolumeSnapshotClient struct {
	GotListOpts []client.ListOption
	Items       []snapshotv1.VolumeSnapshot
	ListErr     error

	DeletedObjs []*snapshotv1.VolumeSnapshot
	DeleteErr   error
}

func (m *mockVolumeSnapshotClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	panic("create should not be called")
}

func (m *mockVolumeSnapshotClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.GotListOpts = opts
	list.(*snapshotv1.VolumeSnapshotList).Items = m.Items
	return m.ListErr
}

func (m *mockVolumeSnapshotClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.DeletedObjs = append(m.DeletedObjs, obj.(*snapshotv1.VolumeSnapshot))
	return m.DeleteErr
}

func TestVolumeSnapshotControl_DeleteOldSnapshots(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path - custom limit", func(t *testing.T) {
		now := time.Now()
		const (
			limit      = 5
			additional = 3
		)

		var mClient mockVolumeSnapshotClient
		for i := 0; i < limit+additional; i++ {
			creation := metav1.NewTime(now.Add(time.Duration(i) * time.Second))
			mClient.Items = append(mClient.Items, snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{Name: strconv.Itoa(i)},
				Status: &snapshotv1.VolumeSnapshotStatus{
					CreationTime: &creation,
				},
			})
		}

		// Nil status should be ignored.
		mClient.Items = append(mClient.Items, snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{Name: "should be filtered out 1"},
			Status:     nil,
		})
		// Nil status.creationTime should be ignored.
		mClient.Items = append(mClient.Items, snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{Name: "should be filtered out 2"},
			Status:     &snapshotv1.VolumeSnapshotStatus{},
		})

		lo.Shuffle(mClient.Items)

		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Name = "agoric"
		crd.Namespace = "default"
		crd.Spec.Limit = int32(limit)

		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		err := control.DeleteOldSnapshots(ctx, nopLogger, &crd)

		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Zero(t, listOpt.Limit)
		require.Equal(t, "default", listOpt.Namespace)
		require.Equal(t, "cosmos.bharvest/source=agoric", listOpt.LabelSelector.String())

		require.EqualValues(t, additional, len(mClient.DeletedObjs))

		got := lo.Map(mClient.DeletedObjs, func(item *snapshotv1.VolumeSnapshot, _ int) string {
			return item.Name
		})
		require.ElementsMatch(t, []string{"0", "1", "2"}, got)
	})

	t.Run("happy path - default limit", func(t *testing.T) {
		now := time.Now()
		const total = 5

		var mClient mockVolumeSnapshotClient
		for i := 0; i < total; i++ {
			creation := metav1.NewTime(now.Add(time.Duration(i) * time.Second))
			mClient.Items = append(mClient.Items, snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{Name: strconv.Itoa(i)},
				Status: &snapshotv1.VolumeSnapshotStatus{
					CreationTime: &creation,
				},
			})
		}

		lo.Shuffle(mClient.Items)

		var crd cosmosalpha.ScheduledVolumeSnapshot
		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		err := control.DeleteOldSnapshots(ctx, nopLogger, &crd)

		require.NoError(t, err)

		require.EqualValues(t, 2, len(mClient.DeletedObjs))

		got := lo.Map(mClient.DeletedObjs, func(item *snapshotv1.VolumeSnapshot, _ int) string {
			return item.Name
		})
		require.ElementsMatch(t, []string{"0", "1"}, got)
	})

	t.Run("happy path - under limit", func(t *testing.T) {
		now := metav1.Now()
		const total = 5

		var mClient mockVolumeSnapshotClient
		for i := 0; i < total; i++ {
			mClient.Items = append(mClient.Items, snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{Name: strconv.Itoa(i)},
				Status: &snapshotv1.VolumeSnapshotStatus{
					CreationTime: &now,
				},
			})
		}

		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Spec.Limit = total + 1
		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		err := control.DeleteOldSnapshots(ctx, nopLogger, &crd)

		require.NoError(t, err)
		require.Empty(t, mClient.DeletedObjs)
	})

	t.Run("happy path - no items", func(t *testing.T) {
		var mClient mockVolumeSnapshotClient
		var crd cosmosalpha.ScheduledVolumeSnapshot
		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		err := control.DeleteOldSnapshots(ctx, nopLogger, &crd)

		require.NoError(t, err)
		require.Empty(t, mClient.DeletedObjs)
	})

	t.Run("list error", func(t *testing.T) {
		var mClient mockVolumeSnapshotClient
		mClient.ListErr = errors.New("boom")
		var crd cosmosalpha.ScheduledVolumeSnapshot
		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		err := control.DeleteOldSnapshots(ctx, nopLogger, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "list volume snapshots: boom")
	})

	t.Run("delete errors", func(t *testing.T) {
		now := metav1.Now()
		const total = 3

		var mClient mockVolumeSnapshotClient
		mClient.DeleteErr = errors.New("oops")
		for i := 0; i < total; i++ {
			mClient.Items = append(mClient.Items, snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{Name: strconv.Itoa(i)},
				Status: &snapshotv1.VolumeSnapshotStatus{
					CreationTime: &now,
				},
			})
		}

		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Spec.Limit = 1
		control := NewVolumeSnapshotControl(&mClient, panicFilter)
		err := control.DeleteOldSnapshots(ctx, nopLogger, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "delete 1: oops\ndelete 0: oops")
	})
}
