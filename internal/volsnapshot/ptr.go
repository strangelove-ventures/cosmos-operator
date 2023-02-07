package volsnapshot

import "github.com/samber/lo"

func ptr[T any](v T) *T {
	return &v
}

func ptrSlice[T any](s []T) []*T {
	return lo.Map(s, func(element T, _ int) *T { return &element })
}
