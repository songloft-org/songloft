package services

import (
	"testing"

	"songloft/internal/models"
	"songloft/internal/version"
)

func withVersionInfo(t *testing.T, versionValue, gitCommit, buildTime, buildType string) {
	t.Helper()

	oldVersion := version.Version
	oldGitCommit := version.GitCommit
	oldBuildTime := version.BuildTime
	oldBuildType := version.BuildType

	version.Version = versionValue
	version.GitCommit = gitCommit
	version.BuildTime = buildTime
	version.BuildType = buildType

	t.Cleanup(func() {
		version.Version = oldVersion
		version.GitCommit = oldGitCommit
		version.BuildTime = oldBuildTime
		version.BuildType = oldBuildType
	})
}

func TestIsNewerVersionReleaseUsesVersionNumber(t *testing.T) {
	withVersionInfo(t, "2.9.6", "abc123", "2026-07-01_00:00:00", "")

	svc := NewUpgradeService()
	sameVersionDifferentCommit := &models.RemoteVersionInfo{
		Version:   "v2.9.6",
		GitCommit: "def456",
		BuildTime: "2026-07-02_00:00:00",
	}
	if svc.isNewerVersion(versionTypeStable, sameVersionDifferentCommit) {
		t.Fatal("same release version with different commit/build time should not be newer")
	}

	newerVersion := &models.RemoteVersionInfo{
		Version:   "v2.9.7",
		GitCommit: "def456",
		BuildTime: "2026-07-02_00:00:00",
	}
	if !svc.isNewerVersion(versionTypeStable, newerVersion) {
		t.Fatal("higher release version should be newer")
	}
}

func TestIsNewerVersionDevUsesBuildTime(t *testing.T) {
	withVersionInfo(t, "dev", "abc123", "2026-07-01_10:00:00", "lite")

	svc := NewUpgradeService()
	olderBuild := &models.RemoteVersionInfo{
		Version:   "dev",
		GitCommit: "def456",
		BuildTime: "2026-07-01_09:00:00",
	}
	if svc.isNewerVersion(versionTypeDev, olderBuild) {
		t.Fatal("older dev build time should not be newer")
	}

	sameCommitNewerBuild := &models.RemoteVersionInfo{
		Version:   "dev",
		GitCommit: "abc123",
		BuildTime: "2026-07-01_11:00:00",
	}
	if svc.isNewerVersion(versionTypeDev, sameCommitNewerBuild) {
		t.Fatal("same git commit should not be newer even with later build time")
	}

	differentCommitNewerBuild := &models.RemoteVersionInfo{
		Version:   "dev",
		GitCommit: "def456",
		BuildTime: "2026-07-01_11:00:00",
	}
	if !svc.isNewerVersion(versionTypeDev, differentCommitNewerBuild) {
		t.Fatal("different commit with newer build time should be newer")
	}
}

func TestIsNewerVersionRejectsCrossChannel(t *testing.T) {
	withVersionInfo(t, "dev", "abc123", "2026-07-01_10:00:00", "")

	svc := NewUpgradeService()
	stableBuild := &models.RemoteVersionInfo{
		Version:   "v9.9.9",
		GitCommit: "def456",
		BuildTime: "2026-07-01_11:00:00",
	}
	if svc.isNewerVersion(versionTypeStable, stableBuild) {
		t.Fatal("dev runtime should not consider stable release as an update")
	}
	if err := svc.ValidateVersionTypeForUpgrade(versionTypeStable); err == nil {
		t.Fatal("dev runtime should reject stable upgrade request")
	}
}

func TestCurrentBuildTypeDefaultsToFull(t *testing.T) {
	withVersionInfo(t, "2.9.6", "abc123", "2026-07-01_10:00:00", "")

	svc := NewUpgradeService()
	if got := svc.CurrentBuildType(); got != buildTypeFull {
		t.Fatalf("CurrentBuildType() = %q, want %q", got, buildTypeFull)
	}
}
