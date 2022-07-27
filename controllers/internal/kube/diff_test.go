package kube

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

	t.Run("simple updates", func(t *testing.T) {
		// Purposefully unordered.
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-11", Labels: ordinalLabels(11)}, // tests for numeric (not lexical) sorting
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Resources: corev1.ResourceRequirements{
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2G")},
							Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("750M")},
						}},
					},
				},
			},
		}

		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						// Different resource requirements.
						{Resources: corev1.ResourceRequirements{
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1G")},
							Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("500M")},
						}},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-11", Labels: ordinalLabels(11), Annotations: map[string]string{"test": "change"}}, // tests for numeric (not lexical) sorting
			},
		}

		diff := NewDiff(testOrdinalLabel, current, want)

		require.Empty(t, diff.Creates())
		require.Empty(t, diff.Deletes())

		require.Len(t, diff.Updates(), 2)

		require.Equal(t, diff.Updates()[0].Name, "hub-0")
		require.Equal(t, diff.Updates()[1].Name, "hub-11")
	})

	t.Run("combination", func(t *testing.T) {
		current := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0), Annotations: map[string]string{"change": "test"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-5", Labels: ordinalLabels(5)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-6", Labels: ordinalLabels(6)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-7", Labels: ordinalLabels(7)},
			},
		}

		want := []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-0", Labels: ordinalLabels(0)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-1", Labels: ordinalLabels(1)},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "hub-2", Labels: ordinalLabels(2)},
			},
		}

		diff := NewDiff(testOrdinalLabel, current, want)

		require.Len(t, diff.Updates(), 1)
		require.Len(t, diff.Creates(), 2)
		require.Len(t, diff.Deletes(), 3)
	})
}

// Benchmark Updates because it uses reflection.
//
// Past results:
// goos: darwin
// goarch: arm64
// pkg: github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube
// BenchmarkDiff_Updates
// BenchmarkDiff_Updates-10    	   31040	     39790 ns/op	   12814 B/op	     270 allocs/op
func BenchmarkDiff_Updates(b *testing.B) {
	const numPods = 25
	current := lo.Map(lo.Range(numPods), func(_ int, i int) *corev1.Pod {
		name := fmt.Sprintf("benchmark-%d", i)
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: ordinalLabels(i)},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2G")},
						Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("750M")},
					}},
				},
			},
		}
	})
	want := lo.Map(lo.Range(numPods), func(_ int, i int) *corev1.Pod {
		name := fmt.Sprintf("benchmark-%d", i)
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: ordinalLabels(i)},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					// Different resources from current.
					{Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1G")},
						Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("500M")},
					}},
				},
			},
		}
	})

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		diff := NewDiff(testOrdinalLabel, current, want)
		if got := diff.Updates(); len(got) == 0 {
			b.Fatal("want at least 1 update, got 0")
		}
	}
}
