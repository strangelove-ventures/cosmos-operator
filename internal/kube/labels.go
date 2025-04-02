package kube

import (
	"bytes"
	"strconv"

	"golang.org/x/exp/constraints"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Recommended labels. See: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
const (
	ControllerLabel = "app.kubernetes.io/created-by"
	InstanceLabel   = "app.kubernetes.io/instance"
	NameLabel       = "app.kubernetes.io/name"
	VersionLabel    = "app.kubernetes.io/version"
	ComponentLabel  = "app.kubernetes.io/component"

	// OrdinalAnnotation is used to order resources. The value must be a base 10 integer string.
	OrdinalAnnotation = "app.kubernetes.io/ordinal"

	BelongsToLabel = "cosmos.strange.love/belongs-to"
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

// ToLabelKey normalizes val per kubernetes label constraints to a max of 63 characters.
// See: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set.
func ToLabelKey(val string) string {
	return normalizeValue(val, 63, '-', '_', '.', '/')
}

// ToName normalizes val per kubernetes name constraints to a max of 253 characters.
// See: https://unofficial-kubernetes.readthedocs.io/en/latest/concepts/overview/working-with-objects/names/
func ToName(val string) string {
	return normalizeValue(val, 253, '-', '.')
}

// NormalizeMetadata normalizes name, labels, and annotations.
// See: https://unofficial-kubernetes.readthedocs.io/en/latest/concepts/overview/working-with-objects/names/
// See: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set.
func NormalizeMetadata(obj *metav1.ObjectMeta) {
	obj.Name = ToName(obj.Name)

	annots := make(map[string]string)
	for k, v := range obj.Annotations {
		annots[ToLabelKey(k)] = v
	}
	obj.Annotations = annots

	labels := make(map[string]string)
	for k, v := range obj.Labels {
		labels[ToLabelKey(k)] = trimMiddle(v, 63)
	}
	obj.Labels = labels
}

func normalizeValue(val string, limit int, allowed ...byte) string {
	// Select only alphanumeric and allowed characters.
	result := []byte(val)
	j := 0
	for _, char := range []byte(val) {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
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

	return trimMiddle(string(result), limit)
}

// Truncates the middle, trying to preserve prefix and suffix.
func trimMiddle(val string, limit int) string {
	if len(val) <= limit {
		return val
	}

	// Truncate the middle, trying to preserve prefix and suffix.
	left, right := limit/2, limit/2
	if limit%2 != 0 {
		right++
	}
	b := []byte(val)
	return string(append(b[:left], b[len(b)-right:]...))
}
