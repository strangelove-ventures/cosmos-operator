package kube

import "strings"

// ParseImageVersion parses the version (aka tag) out of imageRef.
// As an example, "busybox:stable" would return "stable".
// If no tag, defaults to "latest".
func ParseImageVersion(imageRef string) string {
	parts := strings.Split(imageRef, ":")
	if len(parts) != 2 {
		return "latest"
	}
	v := parts[1]
	if v == "" {
		v = "latest"
	}
	return v
}
