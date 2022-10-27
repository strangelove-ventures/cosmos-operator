package snapshot

func ptr[T any](v T) *T {
	return &v
}

func valOrDefault[T any](v *T, defaultVal *T) *T {
	if v == nil {
		return defaultVal
	}
	return v
}
