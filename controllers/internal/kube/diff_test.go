package kube

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func diffablePod(ordinal, generation int) *corev1.Pod {
	p := new(corev1.Pod)
	p.Name = fmt.Sprintf("pod-%d", ordinal)
	p.Annotations = map[string]string{
		OrdinalAnnotation:            ToIntegerValue(ordinal),
		ControllerRevisionAnnotation: ToIntegerValue(generation),
	}
	return p
}

func TestNewDiff(t *testing.T) {
	t.Parallel()

	const generation = 123

	t.Run("non-unique names", func(t *testing.T) {
		dupeNames := []*corev1.Pod{
			diffablePod(0, generation),
			diffablePod(0, generation),
		}
		resources := []*corev1.Pod{
			diffablePod(0, generation),
		}

		require.Panics(t, func() {
			NewDiff(dupeNames, resources)
		})

		require.Panics(t, func() {
			NewDiff(resources, dupeNames)
		})
	})

	t.Run("missing required annotations", func(t *testing.T) {
		for _, tt := range []struct {
			Annotations map[string]string
		}{
			{nil},
			{map[string]string{
				OrdinalAnnotation:            "value should be a number",
				ControllerRevisionAnnotation: "1",
			}},
			{map[string]string{
				OrdinalAnnotation:            "2",
				ControllerRevisionAnnotation: "value should be a number",
			}},
		} {
			current := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Annotations: tt.Annotations},
				},
			}
			require.Panics(t, func() {
				NewDiff(current, nil)
			}, tt)
		}
	})
}

func TestDiff_CreatesDeletesUpdates(t *testing.T) {
	t.Parallel()

	const generation = 123

	t.Run("simple create", func(t *testing.T) {
		current := []*corev1.Pod{
			diffablePod(0, generation),
		}

		// Purposefully unordered
		want := []*corev1.Pod{
			diffablePod(2, generation),
			diffablePod(0, generation),
			diffablePod(110, generation), // tests for numeric (not lexical) sorting
		}

		diff := NewDiff(current, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
		require.Equal(t, diff.Creates()[0].Name, "pod-2")
		require.Equal(t, diff.Creates()[1].Name, "pod-110")
	})

	t.Run("only create", func(t *testing.T) {
		want := []*corev1.Pod{
			diffablePod(0, generation),
			diffablePod(1, generation),
		}

		diff := NewDiff(nil, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
	})

	t.Run("simple delete", func(t *testing.T) {
		// Purposefully unordered.
		current := []*corev1.Pod{
			diffablePod(0, generation),
			diffablePod(11, generation), // tests for numeric (not lexical) sorting
			diffablePod(2, generation),
		}

		want := []*corev1.Pod{
			diffablePod(0, generation),
		}

		diff := NewDiff(current, want)

		require.Empty(t, diff.Updates())
		require.Empty(t, diff.Creates())

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "pod-2")
		require.Equal(t, diff.Deletes()[1].Name, "pod-11")
	})

	t.Run("simple update", func(t *testing.T) {
		t.Fail()
	})

	t.Run("combination", func(t *testing.T) {
		current := []*corev1.Pod{
			diffablePod(0, generation),
			diffablePod(3, generation),
			diffablePod(4, generation),
		}

		want := []*corev1.Pod{
			diffablePod(0, generation),
			diffablePod(1, generation),
		}

		diff := NewDiff(current, want)

		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 1)
		require.Len(t, diff.Deletes(), 2)
	})
}
