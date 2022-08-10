package fullnode

// ptr returns the pointer for any type.
// In k8s, many specs require a pointer to a scalar.
func ptr[T any](v T) *T {
	return &v
}

func valOrDefault[T any](v *T, defaultFn func() *T) *T {
	if v == nil {
		return defaultFn()
	}
	return v
}
