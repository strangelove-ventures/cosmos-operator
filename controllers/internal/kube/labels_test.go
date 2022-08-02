package kube

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestToLabelValue(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		Input string
		Want  string
	}{
		{"hub-0", "hub-0"},
		{"", ""},

		{"HUB!@+=_.0", "hub_.0"},
		{strings.Repeat("abcde^&*", 60) + "-suffix", "abcdeabcdeabcdeabcdeabcdeabcdeaabcdeabcdeabcdeabcdeabcde-suffix"},

		// Must start and end with alphanumeric character.
		{"#..abc1-_@!", "abc1"},
	} {
		got := ToLabelValue(tt.Input)

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

		{"HUB!@+=_.0", "hub.0"},
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
