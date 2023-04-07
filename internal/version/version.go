package version

// version is the version of the build.
// Set via ldflags at build time.
// See Dockerfile, Makefile, and .github/workflows/release.yaml.
var version = ""

// Get returns the version of the build or "latest" if the version is empty.
func Get() string {
	if version == "" {
		return "latest"
	}
	return version
}
