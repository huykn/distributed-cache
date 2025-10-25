package distributedcache

import (
	"testing"
)

func TestGetVersionInfo(t *testing.T) {
	versionInfo := GetVersionInfo()

	if versionInfo.Version == "" {
		t.Error("Version should not be empty")
	}

	if versionInfo.Version != Version {
		t.Errorf("Expected version %s, got %s", Version, versionInfo.Version)
	}
}

func TestVersionConstant(t *testing.T) {
	if Version == "" {
		t.Error("Version constant should not be empty")
	}

	// Version should follow semantic versioning format (basic check)
	if len(Version) < 5 {
		t.Errorf("Version %s seems too short, expected format like '1.0.0'", Version)
	}
}

func TestVersionInfoStruct(t *testing.T) {
	versionInfo := VersionInfo{
		Version:   "1.0.0",
		GoVersion: "go1.21",
	}

	if versionInfo.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", versionInfo.Version)
	}

	if versionInfo.GoVersion != "go1.21" {
		t.Errorf("Expected GoVersion go1.21, got %s", versionInfo.GoVersion)
	}
}
