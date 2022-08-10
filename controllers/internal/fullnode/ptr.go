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

func valOrDefault[T any](v *T, defaultFn func() *T) *T {
	if v == nil {
		return defaultFn()
	}
	return v
}
