package kube

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ordinalAnnotations(n int) map[string]string {
	return map[string]string{OrdinalAnnotation: strconv.Itoa(n)}
}

func TestNewDiff(t *testing.T) {
	t.Parallel()

	t.Run("non-unique names", func(t *testing.T) {
		dupeNames := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
		}
		resources := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Annotations: ordinalAnnotations(2)},
			},
		}

		require.Panics(t, func() {
			NewDiff(dupeNames, resources)
		})

		require.Panics(t, func() {
			NewDiff(resources, dupeNames)
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
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Annotations: map[string]string{OrdinalAnnotation: "value should be a number"}},
			},
		}

		require.Panics(t, func() {
			NewDiff(current, want)
		})
	})
}

func TestDiff_CreatesDeletesUpdates(t *testing.T) {
	t.Parallel()

	t.Run("simple create", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
		}

		// Purposefully unordered
		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Annotations: ordinalAnnotations(2)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-110", Annotations: ordinalAnnotations(110)}, // tests for numeric (not lexical) sorting
			},
		}

		diff := NewDiff(current, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
		require.Equal(t, diff.Creates()[0].Name, "hub-2")
		require.Equal(t, diff.Creates()[1].Name, "hub-110")
	})

	t.Run("only create", func(t *testing.T) {
		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-1", Annotations: ordinalAnnotations(1)},
			},
		}

		diff := NewDiff(nil, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
	})

	t.Run("simple delete", func(t *testing.T) {
		// Purposefully unordered.
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-11", Annotations: ordinalAnnotations(11)}, // tests for numeric (not lexical) sorting
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Annotations: ordinalAnnotations(2)},
			},
		}

		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
		}

		diff := NewDiff(current, want)

		require.Empty(t, diff.Updates())
		require.Empty(t, diff.Creates())

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "hub-2")
		require.Equal(t, diff.Deletes()[1].Name, "hub-11")
	})

	t.Run("combination", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-3", Annotations: ordinalAnnotations(3)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-4", Annotations: ordinalAnnotations(4)},
			},
		}

		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Annotations: ordinalAnnotations(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-1", Annotations: ordinalAnnotations(1)},
			},
		}

		diff := NewDiff(current, want)

		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 1)
		require.Len(t, diff.Deletes(), 2)
	})
}
