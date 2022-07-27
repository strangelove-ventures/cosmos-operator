package kube

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToLabelValue(t *testing.T) {
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
