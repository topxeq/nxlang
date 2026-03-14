package main

// Version information for Nxlang
// These values should be updated for each release
const (
	VersionMajor = 1
	VersionMinor = 0
	VersionPatch = 0
	VersionMeta  = "dev" // Set to "" for release builds
)

// Version returns the version string
func Version() string {
	v := "v1.0.0"
	if VersionMeta != "" {
		v += "-" + VersionMeta
	}
	return v
}
