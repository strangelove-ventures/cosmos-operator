package kube

// Nonstandard but common annotations
const (
	// OrdinalAnnotation value is a sequential integer such as "0", "1", "2". It represents a resource's unique position
	// within a set of resources. It aids in creating resources similar to a StatefulSet.
	OrdinalAnnotation = "app.kubernetes.io/ordinal"
)
