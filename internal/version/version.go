package version

// gitTag is the gitTag of the build.
// Set via ldflags at build time.
// Used for docker image.
// See Dockerfile, Makefile, and .github/workflows/release.yaml.
var gitTag = ""

// Get returns the gitTag of the build or "latest" if the gitTag is empty.
func Get() string {
	if gitTag == "" {
		return "latest"
	}
	return gitTag
}
