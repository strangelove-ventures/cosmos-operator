package cosmos

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	const ordinalLabel = "ordinal"

	labels := func(n int) map[string]string {
		return map[string]string{ordinalLabel: strconv.Itoa(n)}
	}

	t.Run("simple create", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: labels(0)},
			},
		}

		// Purposefully unordered
		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Labels: labels(2)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: labels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-110", Labels: labels(110)}, // tests for numeric (not lexical) sorting
			},
		}

		diff := NewDiff(ordinalLabel, current, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
		require.Equal(t, diff.Creates()[0].Name, "hub-2")
		require.Equal(t, diff.Creates()[1].Name, "hub-110")
	})

	t.Run("simple delete", func(t *testing.T) {
		// Purposefully unordered.
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: labels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-11", Labels: labels(11)}, // tests for numeric (not lexical) sorting
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Labels: labels(2)},
			},
		}

		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: labels(0)},
			},
		}

		diff := NewDiff(ordinalLabel, current, want)

		require.Empty(t, diff.Updates())
		require.Empty(t, diff.Creates())

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "hub-2")
		require.Equal(t, diff.Deletes()[1].Name, "hub-11")
	})

	t.Run("combination", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: labels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-3", Labels: labels(3)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-4", Labels: labels(4)},
			},
		}

		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: labels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-1", Labels: labels(1)},
			},
		}

		diff := NewDiff(ordinalLabel, current, want)

		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 1)
		require.Len(t, diff.Deletes(), 2)
	})

	t.Run("malformed resources", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0"}, // missing label
			},
		}
		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Labels: map[string]string{ordinalLabel: "value should be a number"}},
			},
		}

		require.Panics(t, func() {
			NewDiff(ordinalLabel, current, want)
		})
	})
}
