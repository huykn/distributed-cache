package distributedcache

// Version is the current version of the distributed-cache library.
const Version = "v1.0.2"

// VersionInfo provides version information.
type VersionInfo struct {
	Version   string
	GoVersion string
}

// GetVersionInfo returns the current version information.
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version: Version,
	}
}
