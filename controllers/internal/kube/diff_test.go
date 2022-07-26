package kube

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testOrdinalLabel = "ordinal"

func ordinalLabels(n int) map[string]string {
	return map[string]string{testOrdinalLabel: strconv.Itoa(n)}
}

func TestNewDiff(t *testing.T) {
	t.Parallel()

	t.Run("non-unique names", func(t *testing.T) {
		dupeNames := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
		}
		resources := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Labels: ordinalLabels(2)},
			},
		}

		require.Panics(t, func() {
			NewDiff(testOrdinalLabel, dupeNames, resources)
		})

		require.Panics(t, func() {
			NewDiff(testOrdinalLabel, resources, dupeNames)
		})
	})

	t.Run("missing required labels", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0"}, // missing label
			},
		}
		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Labels: map[string]string{testOrdinalLabel: "value should be a number"}},
			},
		}

		require.Panics(t, func() {
			NewDiff(testOrdinalLabel, current, want)
		})
	})
}

func TestDiff_CreatesDeletesUpdates(t *testing.T) {
	t.Parallel()

	t.Run("simple create", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
		}

		// Purposefully unordered
		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Labels: ordinalLabels(2)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-110", Labels: ordinalLabels(110)}, // tests for numeric (not lexical) sorting
			},
		}

		diff := NewDiff(testOrdinalLabel, current, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
		require.Equal(t, diff.Creates()[0].Name, "hub-2")
		require.Equal(t, diff.Creates()[1].Name, "hub-110")
	})

	t.Run("only create", func(t *testing.T) {
		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-1", Labels: ordinalLabels(1)},
			},
		}

		diff := NewDiff(testOrdinalLabel, nil, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
	})

	t.Run("simple delete", func(t *testing.T) {
		// Purposefully unordered.
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-11", Labels: ordinalLabels(11)}, // tests for numeric (not lexical) sorting
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Labels: ordinalLabels(2)},
			},
		}

		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
		}

		diff := NewDiff(testOrdinalLabel, current, want)

		require.Empty(t, diff.Updates())
		require.Empty(t, diff.Creates())

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "hub-2")
		require.Equal(t, diff.Deletes()[1].Name, "hub-11")
	})

	t.Run("combination", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-3", Labels: ordinalLabels(3)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-4", Labels: ordinalLabels(4)},
			},
		}

		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-1", Labels: ordinalLabels(1)},
			},
		}

		diff := NewDiff(testOrdinalLabel, current, want)

		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 1)
		require.Len(t, diff.Deletes(), 2)
	})
}

func TestDiff_IsDirty(t *testing.T) {
	t.Parallel()

	t.Fatal("TODO")

	//t.Run("clean state", func(t *testing.T) {
	//	current := []*corev1.Pod{
	//		{
	//			ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
	//			Status: metav1.Status{
	//				Status:  "",
	//				Message: "",
	//				Reason:  "",
	//				Details: nil,
	//				Code:    0,
	//			},
	//		},
	//	}
	//
	//	// Purposefully unordered
	//	want := []*corev1.Pod{
	//		{
	//			ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
	//		},
	//		{
	//			ObjectMeta: metav1.ObjectMeta{Name: "hub-1", Labels: ordinalLabels(0)},
	//		},
	//	}
	//})
	//
	//t.Run("dirty", func(t *testing.T) {
	//
	//})
}
