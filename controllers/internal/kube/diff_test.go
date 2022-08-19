package kube

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testOrdinalAnnotation = "ordinal"
	testRevisionLabel     = "revision"
)

func diffablePod(ordinal int, revision string) *corev1.Pod {
	p := new(corev1.Pod)
	p.Name = fmt.Sprintf("pod-%d", ordinal)
	p.Annotations = map[string]string{
		testOrdinalAnnotation: ToIntegerValue(ordinal),
	}
	p.Labels = map[string]string{
		testRevisionLabel: revision,
	}
	return p
}

func TestNewDiff(t *testing.T) {
	t.Parallel()

	const revision = "_revision_"

	t.Run("non-unique names", func(t *testing.T) {
		dupeNames := []*corev1.Pod{
			diffablePod(0, revision),
			diffablePod(0, revision),
		}
		resources := []*corev1.Pod{
			diffablePod(0, revision),
		}

		require.Panics(t, func() {
			NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, dupeNames, resources)
		})

		require.Panics(t, func() {
			NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, resources, dupeNames)
		})
	})

	t.Run("missing required annotations and labels", func(t *testing.T) {
		for _, tt := range []struct {
			OrdinalValue  string
			RevisionValue string
		}{
			{"", "revision"},
			{"should be a number", "revision"},
			{"1", ""},
		} {
			bad := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pod-0",
						Annotations: map[string]string{testOrdinalAnnotation: tt.OrdinalValue},
						Labels:      map[string]string{testRevisionLabel: tt.RevisionValue},
					},
				},
			}
			good := []*corev1.Pod{
				diffablePod(0, "_new_resource_"),
			}

			// A blank revision is ok for the exiting resources. Future proofs the unlikely event we change the revision label.
			if tt.RevisionValue != "" {
				require.Panics(t, func() {
					NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, bad, good)
				}, tt)
			}

			// Test the inverse.
			require.Panics(t, func() {
				NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, good, bad)
			}, tt)
		}
	})
}

func TestDiff_CreatesDeletesUpdates(t *testing.T) {
	t.Parallel()

	const revision = "_revision_"

	t.Run("simple create", func(t *testing.T) {
		current := []*corev1.Pod{
			diffablePod(0, revision),
		}

		// Purposefully unordered
		want := []*corev1.Pod{
			diffablePod(2, revision),
			diffablePod(0, revision),
			diffablePod(110, revision), // tests for numeric (not lexical) sorting
		}

		diff := NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, current, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
		require.Equal(t, diff.Creates()[0].Name, "pod-2")
		require.Equal(t, diff.Creates()[1].Name, "pod-110")
	})

	t.Run("only create", func(t *testing.T) {
		want := []*corev1.Pod{
			diffablePod(0, revision),
			diffablePod(1, revision),
		}

		diff := NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, nil, want)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)
	})

	t.Run("simple delete", func(t *testing.T) {
		// Purposefully unordered.
		current := []*corev1.Pod{
			diffablePod(0, revision),
			diffablePod(11, revision), // tests for numeric (not lexical) sorting
			diffablePod(2, revision),
		}

		want := []*corev1.Pod{
			diffablePod(0, revision),
		}

		diff := NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, current, want)

		require.Empty(t, diff.Updates())
		require.Empty(t, diff.Creates())

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "pod-2")
		require.Equal(t, diff.Deletes()[1].Name, "pod-11")
	})

	t.Run("simple update", func(t *testing.T) {
		// Purposefully unordered.
		current := []*corev1.Pod{
			diffablePod(22, revision), // tests for numeric (not lexical) sorting
			diffablePod(2, revision),
		}

		want := []*corev1.Pod{
			diffablePod(22, "_new_version_"),
			diffablePod(2, "_new_version_"),
		}

		diff := NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, current, want)

		require.Empty(t, diff.Creates())
		require.Empty(t, diff.Deletes())

		require.Len(t, diff.Updates(), 2)
		require.Equal(t, diff.Updates()[0].Name, "pod-2")
		require.Equal(t, diff.Updates()[1].Name, "pod-22")
	})

	t.Run("combination", func(t *testing.T) {
		current := []*corev1.Pod{
			diffablePod(0, revision),
			diffablePod(3, revision),
			diffablePod(4, revision),
		}

		want := []*corev1.Pod{
			diffablePod(0, "_new_version_"),
			diffablePod(1, revision),
		}

		diff := NewOrdinalDiff(testOrdinalAnnotation, testRevisionLabel, current, want)

		require.Len(t, diff.Updates(), 1)
		require.Equal(t, "pod-0", diff.Updates()[0].Name)

		require.Len(t, diff.Creates(), 1)
		require.Equal(t, "pod-1", diff.Creates()[0].Name)

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "pod-3")
		require.Equal(t, diff.Deletes()[1].Name, "pod-4")
	})
}
