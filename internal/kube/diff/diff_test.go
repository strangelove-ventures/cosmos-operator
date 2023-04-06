package kube

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testOrdinalAnnotation = "ordinal"
	testRevisionLabel     = "revision"
)

type diffAdapter struct {
	*corev1.Pod
	revision string
	ordinal  int64
}

func (d diffAdapter) Revision() string      { return d.revision }
func (d diffAdapter) Ordinal() int64        { return d.ordinal }
func (d diffAdapter) Object() client.Object { return d.Pod }

func diffablePod(ordinal int, revision string) diffAdapter {
	p := new(corev1.Pod)
	p.Name = fmt.Sprintf("pod-%d", ordinal)
	p.Namespace = "default"
	return diffAdapter{Pod: p, ordinal: int64(ordinal), revision: revision}
}

func TestOrdinalDiff_CreatesDeletesUpdates(t *testing.T) {
	t.Parallel()

	const namespace = "default"

	t.Run("create", func(t *testing.T) {
		current := []*corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: namespace}},
		}

		// Purposefully unordered
		want := []Resource{
			diffablePod(110, "rev-110"),
			diffablePod(1, "rev-1"),
			diffablePod(0, "rev"),
		}

		diff := New(current, want)

		//require.Empty(t, diff.Deletes())
		//require.Empty(t, diff.Updates())

		require.Len(t, diff.Creates(), 2)

		require.Equal(t, "pod-1", diff.Creates()[0].Name)
		require.Equal(t, "rev-1", diff.Creates()[0].Labels["app.kubernetes.io/revision"])
		require.Equal(t, "1", diff.Creates()[0].Annotations["app.kubernetes.io/ordinal"])

		require.Equal(t, "pod-110", diff.Creates()[1].Name)
		require.Equal(t, "rev-110", diff.Creates()[1].Labels["app.kubernetes.io/revision"])
		require.Equal(t, "110", diff.Creates()[1].Annotations["app.kubernetes.io/ordinal"])
	})

	//t.Run("only create", func(t *testing.T) {
	//	want := []*corev1.Pod{
	//		revisionDiffablePod(0, revision),
	//		revisionDiffablePod(1, revision),
	//	}
	//
	//	diff := NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, nil, want)
	//
	//	require.Empty(t, diff.Deletes())
	//	require.Empty(t, diff.Updates())
	//
	//	require.Len(t, diff.Creates(), 2)
	//})
	//
	//t.Run("simple delete", func(t *testing.T) {
	//	// Purposefully unordered.
	//	current := []*corev1.Pod{
	//		revisionDiffablePod(0, revision),
	//		revisionDiffablePod(11, revision), // tests for numeric (not lexical) sorting
	//		revisionDiffablePod(2, revision),
	//	}
	//
	//	want := []*corev1.Pod{
	//		revisionDiffablePod(0, revision),
	//	}
	//
	//	diff := NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, current, want)
	//
	//	require.Empty(t, diff.Updates())
	//	require.Empty(t, diff.Creates())
	//
	//	require.Len(t, diff.Deletes(), 2)
	//	require.Equal(t, diff.Deletes()[0].Name, "pod-2")
	//	require.Equal(t, diff.Deletes()[1].Name, "pod-11")
	//})
	//
	//t.Run("simple update", func(t *testing.T) {
	//	pod1 := revisionDiffablePod(2, revision)
	//	pod1.SetUID("uuid1")
	//	pod1.SetResourceVersion("1")
	//	pod1.SetGeneration(100)
	//
	//	pod2 := revisionDiffablePod(22, revision)
	//	pod2.SetUID("uuid2")
	//	pod2.SetResourceVersion("2")
	//	pod2.SetGeneration(200)
	//
	//	// Purposefully unordered to test for numeric (vs lexical) sorting.
	//	current := []*corev1.Pod{pod2, pod1}
	//
	//	want := []*corev1.Pod{
	//		revisionDiffablePod(22, "_new_version_"),
	//		revisionDiffablePod(2, "_new_version_"),
	//	}
	//
	//	diff := NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, current, want)
	//
	//	require.Empty(t, diff.Creates())
	//	require.Empty(t, diff.Deletes())
	//
	//	require.Len(t, diff.Updates(), 2)
	//	require.Equal(t, diff.Updates()[0].Name, "pod-2")
	//	require.Equal(t, diff.Updates()[1].Name, "pod-22")
	//
	//	gotPod := diff.Updates()[1]
	//	require.Equal(t, "2", gotPod.GetResourceVersion())
	//	require.EqualValues(t, "uuid2", gotPod.GetUID())
	//	require.EqualValues(t, 200, gotPod.GetGeneration())
	//})
	//
	//t.Run("combination", func(t *testing.T) {
	//	current := []*corev1.Pod{
	//		revisionDiffablePod(0, revision),
	//		revisionDiffablePod(3, revision),
	//		revisionDiffablePod(4, revision),
	//	}
	//
	//	want := []*corev1.Pod{
	//		revisionDiffablePod(0, "_new_version_"),
	//		revisionDiffablePod(1, revision),
	//	}
	//
	//	for _, tt := range []struct {
	//		TestName string
	//		Diff     *Diff[*corev1.Pod]
	//	}{
	//		{"ordinal", NewOrdinalRevisionDiff(testOrdinalAnnotation, testRevisionLabel, current, want)},
	//		{"non-ordinal", NewRevisionDiff(testRevisionLabel, current, want)},
	//	} {
	//		diff := tt.Diff
	//		require.Len(t, diff.Updates(), 1, tt.TestName)
	//		require.Equal(t, "pod-0", diff.Updates()[0].Name, tt.TestName)
	//
	//		require.Len(t, diff.Creates(), 1, tt.TestName)
	//		require.Equal(t, "pod-1", diff.Creates()[0].Name, tt.TestName)
	//
	//		deletes := lo.Map(diff.Deletes(), func(p *corev1.Pod, _ int) string {
	//			return p.Name
	//		})
	//		require.Len(t, deletes, 2)
	//		require.ElementsMatch(t, []string{"pod-3", "pod-4"}, deletes)
	//	}
	//})
}
