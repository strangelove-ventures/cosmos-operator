package fullnode

import "github.com/samber/lo"

// ptr returns the pointer for any type.
// In k8s, many specs require a pointer to a scalar.
func ptr[T any](v T) *T {
	return &v
}

func ptrSlice[T any](s []T) []*T {
	return lo.Map(s, func(element T, _ int) *T { return &element })
}

func valSlice[T any](s []*T) []T {
	return lo.Map(s, func(element *T, _ int) T { return *element })
}

func valOrDefault[T any](v *T, defaultVal *T) *T {
	if v == nil {
		return defaultVal
	}
	return v
}

func sliceOrDefault[T any](slice []T, defaultSlice []T) []T {
	if len(slice) == 0 {
		return defaultSlice
	}
	return slice
}
