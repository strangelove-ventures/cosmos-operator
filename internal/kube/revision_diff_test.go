package kube

import (
	"fmt"
	"testing"

	"github.com/samber/lo"
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

func TestNewOrdinalDiff(t *testing.T) {
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
			NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, dupeNames, resources)
		})

		require.Panics(t, func() {
			NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, resources, dupeNames)
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
					NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, bad, good)
				}, tt)
			}

			// Test the inverse.
			require.Panics(t, func() {
				NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, good, bad)
			}, tt)
		}
	})
}

func TestNewDiff(t *testing.T) {
	resources := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pod-0",
				Labels: map[string]string{testRevisionLabel: "revision"},
			},
		},
	}
	require.NotPanics(t, func() {
		NewRevisionDiff(testRevisionLabel, resources, resources)
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

		diff := NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, current, want)

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

		diff := NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, nil, want)

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

		diff := NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, current, want)

		require.Empty(t, diff.Updates())
		require.Empty(t, diff.Creates())

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "pod-2")
		require.Equal(t, diff.Deletes()[1].Name, "pod-11")
	})

	t.Run("simple update", func(t *testing.T) {
		pod1 := diffablePod(2, revision)
		pod1.SetUID("uuid1")
		pod1.SetResourceVersion("1")
		pod1.SetGeneration(100)

		pod2 := diffablePod(22, revision)
		pod2.SetUID("uuid2")
		pod2.SetResourceVersion("2")
		pod2.SetGeneration(200)

		// Purposefully unordered to test for numeric (vs lexical) sorting.
		current := []*corev1.Pod{pod2, pod1}

		want := []*corev1.Pod{
			diffablePod(22, "_new_version_"),
			diffablePod(2, "_new_version_"),
		}

		diff := NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, current, want)

		require.Empty(t, diff.Creates())
		require.Empty(t, diff.Deletes())

		require.Len(t, diff.Updates(), 2)
		require.Equal(t, diff.Updates()[0].Name, "pod-2")
		require.Equal(t, diff.Updates()[1].Name, "pod-22")

		gotPod := diff.Updates()[1]
		require.Equal(t, "2", gotPod.GetResourceVersion())
		require.EqualValues(t, "uuid2", gotPod.GetUID())
		require.EqualValues(t, 200, gotPod.GetGeneration())
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

		for _, tt := range []struct {
			TestName string
			Diff     *RevisionDiff[*corev1.Pod]
		}{
			{"ordinal", NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, current, want)},
			{"non-ordinal", NewRevisionDiff(testRevisionLabel, current, want)},
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
