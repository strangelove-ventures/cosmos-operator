package kube

import (
	"bytes"
	"strconv"
	"strings"

	"golang.org/x/exp/constraints"
)

// Recommended labels. See: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
const (
	ControllerLabel = "app.kubernetes.io/created-by"
	InstanceLabel   = "app.kubernetes.io/instance"
	NameLabel       = "app.kubernetes.io/name"
	VersionLabel    = "app.kubernetes.io/version"
	ComponentLabel  = "app.kubernetes.io/component"

	// RevisionLabel denotes the resource's revision, typically a hex-encoded hash. Used to detect resource changes for updates.
	RevisionLabel = "app.kubernetes.io/revision"
)

// Fields.
const (
	ControllerOwnerField = ".metadata.controller"
)

// ToIntegerValue converts n to a base 10 integer string.
func ToIntegerValue[T constraints.Signed](n T) string {
	return strconv.FormatInt(int64(n), 10)
}

// MustToInt converts s to int64 or panics on failure.
func MustToInt(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}

// ToLabelValue normalizes val per kubernetes label constraints to a max of 63 characters.
// This function lowercases even though uppercase is allowed.
// See: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set.
func ToLabelValue(val string) string {
	return normalizeValue(val, 63, '-', '_', '.')
}

// ToName normalizes val per kubernetes name constraints to a max of 253 characters.
// This function lowercases even though uppercase is allowed.
// See: https://unofficial-kubernetes.readthedocs.io/en/latest/concepts/overview/working-with-objects/names/
func ToName(val string) string {
	return normalizeValue(val, 253, '-', '.')
}

func normalizeValue(val string, limit int, allowed ...byte) string {
	val = strings.ToLower(val)

	// Select only alphanumeric and allowed characters.
	result := []byte(val)
	j := 0
	for _, char := range []byte(val) {
		if (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			(bytes.IndexByte(allowed, char) != -1) {
			result[j] = char
			j++
		}
	}
	result = result[:j]

	// Start and end with alphanumeric only
	result = bytes.TrimLeftFunc(result, func(r rune) bool {
		return bytes.ContainsRune(allowed, r)
	})
	result = bytes.TrimRightFunc(result, func(r rune) bool {
		return bytes.ContainsRune(allowed, r)
	})

	if len(result) <= limit {
		return string(result)
	}

	// Truncate the middle, trying to preserve prefix and suffix.
	left, right := limit/2, limit/2
	if limit%2 != 0 {
		right++
	}
	return string(append(result[:left], result[len(result)-right:]...))
}
