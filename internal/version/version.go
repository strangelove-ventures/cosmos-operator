package version

// version is the version of the build.
// Set via ldflags at build time.
// Used for docker image.
// See Dockerfile, Makefile, and .github/workflows/release.yaml.
var version = ""

// DockerTag returns the version of the build or "latest" if unknown.
func DockerTag() string {
	if version == "" {
		return "latest"
	}
	return version
}

// AppVersion returns the version of the build or "(devel)" if unknown.
func AppVersion() string {
	if version == "" {
		return "(devel)"
	}
	return version
}
