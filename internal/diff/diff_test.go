package diff

import (
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type diffAdapter struct {
	*corev1.Pod
	revision string
	ordinal  int64
}

func (d diffAdapter) Revision() string    { return d.revision }
func (d diffAdapter) Ordinal() int64      { return d.ordinal }
func (d diffAdapter) Object() *corev1.Pod { return d.Pod }

func diffablePod(ordinal int, revision string) diffAdapter {
	p := new(corev1.Pod)
	p.Name = fmt.Sprintf("pod-%d", ordinal)
	p.Namespace = "default"
	return diffAdapter{Pod: p, ordinal: int64(ordinal), revision: revision}
}

func TestOrdinalDiff_CreatesDeletesUpdates(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		current := []*corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default", Labels: map[string]string{revisionLabel: "rev"}}},
		}

		// Purposefully unordered
		want := []Resource[*corev1.Pod]{
			diffablePod(110, "rev-110"),
			diffablePod(1, "rev-1"),
			diffablePod(0, "rev"),
		}

		diff := New(current, want)

		require.Len(t, diff.Creates(), 2)

		require.Equal(t, "pod-1", diff.Creates()[0].Name)
		require.Equal(t, "rev-1", diff.Creates()[0].Labels["app.kubernetes.io/revision"])
		require.Equal(t, "1", diff.Creates()[0].Annotations["app.kubernetes.io/ordinal"])

		require.Equal(t, "pod-110", diff.Creates()[1].Name)
		require.Equal(t, "rev-110", diff.Creates()[1].Labels["app.kubernetes.io/revision"])
		require.Equal(t, "110", diff.Creates()[1].Annotations["app.kubernetes.io/ordinal"])

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())
	})

	t.Run("no current resources", func(t *testing.T) {
		var current []*corev1.Pod
		want := []Resource[*corev1.Pod]{
			diffablePod(0, "rev"),
		}
		diff := New(current, want)

		require.Len(t, diff.Creates(), 1)

		require.Empty(t, diff.Deletes())
		require.Empty(t, diff.Updates())
	})

	t.Run("simple delete", func(t *testing.T) {
		// Purposefully unordered to test lexical sorting.
		existing := []Resource[*corev1.Pod]{
			diffablePod(0, "doesn't matter"),
			diffablePod(11, "doesn't matter"),
			diffablePod(2, "doesn't matter"),
		}
		diff := New(nil, existing)
		current := diff.Creates()

		want := []Resource[*corev1.Pod]{
			diffablePod(0, "doesn't matter"),
		}
		diff = New(current, want)

		require.Len(t, diff.Deletes(), 2)
		require.Equal(t, diff.Deletes()[0].Name, "pod-2")
		require.Equal(t, diff.Deletes()[1].Name, "pod-11")

		require.Empty(t, diff.Creates())
		require.Empty(t, diff.Updates())
	})

	t.Run("updates", func(t *testing.T) {
		pod1 := diffablePod(2, "rev-2")
		pod1.SetGeneration(2)
		pod1.SetUID("uuid2")
		pod1.SetResourceVersion("rv2")
		pod1.SetOwnerReferences([]metav1.OwnerReference{{Name: "owner2"}})

		pod2 := diffablePod(11, "rev-11")
		pod2.SetGeneration(11)
		pod2.SetUID("uuid11")
		pod2.SetResourceVersion("rv11")
		pod2.SetOwnerReferences([]metav1.OwnerReference{{Name: "owner11"}})

		existing := []Resource[*corev1.Pod]{pod1, pod2}
		diff := New(nil, existing)
		current := diff.Creates()

		// Purposefully unordered to test lexical sorting.
		want := []Resource[*corev1.Pod]{
			diffablePod(11, "changed-11"),
			diffablePod(2, "changed-2"),
		}
		diff = New(current, want)

		require.Len(t, diff.Updates(), 2)
		require.Equal(t, "pod-2", diff.Updates()[0].Name)
		require.Equal(t, "changed-2", diff.Updates()[0].Labels["app.kubernetes.io/revision"])
		require.Equal(t, "2", diff.Updates()[0].Annotations["app.kubernetes.io/ordinal"])
		require.Equal(t, int64(2), diff.Updates()[0].Generation)
		require.Equal(t, "uuid2", string(diff.Updates()[0].UID))
		require.Equal(t, "rv2", diff.Updates()[0].ResourceVersion)
		require.Equal(t, "owner2", diff.Updates()[0].OwnerReferences[0].Name)

		require.Equal(t, "pod-11", diff.Updates()[1].Name)
		require.Equal(t, "changed-11", diff.Updates()[1].Labels["app.kubernetes.io/revision"])
		require.Equal(t, "11", diff.Updates()[1].Annotations["app.kubernetes.io/ordinal"])
		require.Equal(t, int64(11), diff.Updates()[1].Generation)
		require.Equal(t, "uuid11", string(diff.Updates()[1].UID))
		require.Equal(t, "rv11", diff.Updates()[1].ResourceVersion)
		require.Equal(t, "owner11", diff.Updates()[1].OwnerReferences[0].Name)

		require.Empty(t, diff.Creates())
		require.Empty(t, diff.Deletes())
	})

	t.Run("combination", func(t *testing.T) {
		existing := []Resource[*corev1.Pod]{
			diffablePod(0, "1"),
			diffablePod(1, "1"),
			diffablePod(2, "1"),
		}
		diff := New(nil, existing)
		current := diff.Creates()

		// Purposefully unordered to test lexical sorting.
		want := []Resource[*corev1.Pod]{
			diffablePod(0, "changed-1"),
			diffablePod(1, "1"), // no change
			// pod-2 is deleted
			diffablePod(3, "doesn't mater"),
		}

		diff = New(current, want)

		created := lo.Map(diff.Creates(), func(pod *corev1.Pod, _ int) string { return pod.Name })
		require.Equal(t, []string{"pod-3"}, created)

		updated := lo.Map(diff.Updates(), func(pod *corev1.Pod, _ int) string { return pod.Name })
		require.Equal(t, []string{"pod-0"}, updated)

		deleted := lo.Map(diff.Deletes(), func(pod *corev1.Pod, _ int) string { return pod.Name })
		require.Equal(t, []string{"pod-2"}, deleted)
	})
}
