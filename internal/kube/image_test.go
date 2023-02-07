package kube

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseImageVersion(t *testing.T) {
	for _, tt := range []struct {
		ImageRef string
		Want     string
	}{
		{"", "latest"},
		{"busybox", "latest"},
		{"busybox:stable", "stable"},
		{"ghcr.io/strangelove-ventures/heighliner/osmosis:v9.0.0", "v9.0.0"},
		{"busybox:", "latest"},
	} {
		got := ParseImageVersion(tt.ImageRef)

		require.Equal(t, tt.Want, got, tt)
	}
}
