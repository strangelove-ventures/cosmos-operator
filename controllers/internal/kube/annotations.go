package kube

const (
	// OrdinalAnnotation denotes the resource's ordinal position.
	// This annotation key must never be changed, or it will be a backward breaking change for operator upgrades.
	OrdinalAnnotation = "app.kubernetes.io/ordinal"
)
