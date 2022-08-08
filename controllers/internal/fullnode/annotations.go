package fullnode

const (
	// OrdinalAnnotation denotes the resource's ordinal position.
	OrdinalAnnotation = "cosmosfullnode.cosmos.strange.love/ordinal"

	// Denotes the resource's revision typically using hex-encoded fnv hash. Used to detect resource changes for updates.
	revisionAnnotation = "cosmosfullnode.cosmos.strange.love/resource-revision"
)
