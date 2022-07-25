package controllers

import "github.com/samber/lo"

func ptrSlice[T any](s []T) []*T {
	return lo.Map(s, func(element T, _ int) *T { return &element })
}
