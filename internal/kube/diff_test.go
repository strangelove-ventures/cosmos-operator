package kube

import (
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func diffablePod(ordinal int) *corev1.Pod {
	p := new(corev1.Pod)
	p.Name = fmt.Sprintf("pod-%d", ordinal)
	p.Annotations = map[string]string{
		testOrdinalAnnotation: ToIntegerValue(ordinal),
	}
	p.Labels = map[string]string{}
	return p
}

func TestDiff_CreatesDeletesUpdates(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		current := []*corev1.Pod{
			diffablePod(0),
		}

		// Purposefully unordered
		want := []*corev1.Pod{
			diffablePod(2),
			diffablePod(0),
			diffablePod(110), // tests for numeric (not lexical) sorting
		}

		diff := NewOrdinalDiff(testOrdinalAnnotation, current, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
		require.Equal(t, diff.Creates()[0].Name, "pod-2")
		require.Equal(t, diff.Creates()[1].Name, "pod-110")
	})

	t.Run("creates brand new resources", func(t *testing.T) {
		want := []*corev1.Pod{
			diffablePod(0),
			diffablePod(1),
		}

		diff := NewOrdinalDiff(testOrdinalAnnotation, nil, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
	})

	t.Run("deletes", func(t *testing.T) {
		// Purposefully unordered.
		current := []*corev1.Pod{
			diffablePod(0),
			diffablePod(11), // tests for numeric (not lexical) sorting
			diffablePod(2),
		}

		want := []*corev1.Pod{
			diffablePod(0), // Should not appear in deletes.
		}

		diff := NewOrdinalDiff(testOrdinalAnnotation, current, want)

		require.Empty(t, diff.Updates())
		require.Empty(t, diff.Creates())

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "pod-2")
		require.Equal(t, diff.Deletes()[1].Name, "pod-11")
	})

	t.Run("updates", func(t *testing.T) {
		pod1 := diffablePod(2)
		pod2 := diffablePod(22)
		pod3 := diffablePod(44)

		current := []*corev1.Pod{pod2, pod1, pod3}

		want := []*corev1.Pod{
			diffablePod(22),
			diffablePod(2),
			pod3.DeepCopy(), // Should not appear in updates.
		}
		// Add changes
		want[0].Labels["foo"] = "bar"
		want[1].Spec.Priority = ptr(int32(100))

		diff := NewOrdinalDiff(testOrdinalAnnotation, current, want)

		require.Empty(t, diff.Creates())
		require.Empty(t, diff.Deletes())

		require.Len(t, diff.Updates(), 2)
		require.Equal(t, diff.Updates()[0].Name, "pod-2")
		require.Equal(t, diff.Updates()[1].Name, "pod-22")
	})

	t.Run("combination", func(t *testing.T) {
		current := []*corev1.Pod{
			diffablePod(0),
			diffablePod(3),
			diffablePod(4),
		}

		want := []*corev1.Pod{
			diffablePod(0),
			diffablePod(1),
		}

		// Cause an update
		want[0].Annotations["foo"] = "bar"

		for _, tt := range []struct {
			TestName string
			Diff     *Diff[*corev1.Pod]
		}{
			{"ordinal", NewOrdinalDiff(testOrdinalAnnotation, current, want)},
			{"non-ordinal", NewDiff(current, want)},
		} {
			diff := tt.Diff
			require.Len(t, diff.Updates(), 1, tt.TestName)
			require.Equal(t, "pod-0", diff.Updates()[0].Name, tt.TestName)

			require.Len(t, diff.Creates(), 1, tt.TestName)
			require.Equal(t, "pod-1", diff.Creates()[0].Name, tt.TestName)

			deletes := lo.Map(diff.Deletes(), func(p *corev1.Pod, _ int) string {
				return p.Name
			})
			require.Len(t, deletes, 2)
			require.ElementsMatch(t, []string{"pod-3", "pod-4"}, deletes)
		}
	})
}
