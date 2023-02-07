package kube

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/strangelove-ventures/cosmos-operator/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToLabelKey(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		Input string
		Want  string
	}{
		{"hub-0", "hub-0"},
		{"", ""},

		{"HUB!@+=_.0", "HUB_.0"},
		{strings.Repeat("abcde^&*", 60) + "-suffix", "abcdeabcdeabcdeabcdeabcdeabcdeaabcdeabcdeabcdeabcdeabcde-suffix"},

		// Must start and end with alphanumeric character.
		{"#..abc1-_@!", "abc1"},
	} {
		got := ToLabelKey(tt.Input)

		require.LessOrEqual(t, len(got), 63)
		require.Equal(t, tt.Want, got, tt)
	}
}

func TestToName(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		Input string
		Want  string
	}{
		{"hub-0", "hub-0"},
		{"", ""},

		{"HUB!@+=_.0", "HUB.0"},
		{strings.Repeat("abcde^&*", 100) + "-suffix", "abcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeaabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcdeabcde-suffix"},

		// Must start and end with alphanumeric character.
		{"#..abc2-_@!", "abc2"},
	} {
		got := ToName(tt.Input)

		require.LessOrEqual(t, len(got), 253)
		require.Equal(t, tt.Want, got, tt)
	}
}

func TestToIntegerValue(t *testing.T) {
	t.Parallel()

	require.Equal(t, "123", ToIntegerValue(123))
	require.Equal(t, "-1", ToIntegerValue(-1))
}

func TestMustToInt(t *testing.T) {
	t.Parallel()

	require.EqualValues(t, 123, MustToInt(ToIntegerValue(123)))

	rand.Seed(time.Now().UnixNano())
	n := rand.Intn(1000)
	require.EqualValues(t, n, MustToInt(fmt.Sprintf("%d", n)))

	for _, badValue := range []string{"", "1.2", "1-2"} {
		require.Panics(t, func() {
			MustToInt(badValue)
		})
	}
}

func TestNormalizeMetadata(t *testing.T) {
	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        strings.Repeat(" name ", 500),
			Annotations: map[string]string{strings.Repeat("annot-key", 500): strings.Repeat("value", 500), "cloud.google.com/neg": `{"ingress": true}`},
			Labels:      map[string]string{strings.Repeat("label-key", 500): strings.Repeat("value", 500)},
		},
	}

	NormalizeMetadata(&obj.ObjectMeta)

	test.RequireValidMetadata(t, obj)
	require.Equal(t, `{"ingress": true}`, obj.Annotations["cloud.google.com/neg"])
}
