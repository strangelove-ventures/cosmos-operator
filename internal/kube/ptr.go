package kube

func ptr[T any](v T) *T {
	return &v
}
