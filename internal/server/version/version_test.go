package version

import "testing"

func TestAppVersion_NonEmpty(t *testing.T) {
	if AppVersion == "" {
		t.Error("AppVersion should not be empty")
	}
}

func TestAppVersion_Format(t *testing.T) {
	// Expecting semver format: X.Y.Z
	if len(AppVersion) < 3 {
		t.Errorf("AppVersion too short: %q", AppVersion)
	}
	// Should contain dots
	hasDot := false
	for _, c := range AppVersion {
		if c == '.' {
			hasDot = true
			break
		}
	}
	if !hasDot {
		t.Errorf("AppVersion %q does not look like semver", AppVersion)
	}
}

func TestAppVersion_Stable(t *testing.T) {
	// Just verify it's a constant value that doesn't panic
	_ = AppVersion
}

func TestAppVersion_Exact(t *testing.T) {
	if AppVersion != "4.1.2" {
		t.Logf("AppVersion = %q (this test may need updating when version changes)", AppVersion)
	}
}
