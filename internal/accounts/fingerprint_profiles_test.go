package accounts

import (
	"testing"
)

func TestDefaultProfilesCount(t *testing.T) {
	if len(DefaultProfiles) != 8 {
		t.Fatalf("DefaultProfiles has %d profiles, want 8", len(DefaultProfiles))
	}
}

func TestDefaultProfilesUniqueNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range DefaultProfiles {
		if seen[p.Name] {
			t.Errorf("duplicate profile name: %s", p.Name)
		}
		seen[p.Name] = true
	}
	if len(seen) != 8 {
		t.Errorf("got %d unique names, want 8", len(seen))
	}
}

func TestDefaultProfilesValidValues(t *testing.T) {
	for _, p := range DefaultProfiles {
		if p.UserAgent == "" {
			t.Errorf("profile %s has empty UserAgent", p.Name)
		}
		if p.ScreenWidth <= 0 || p.ScreenHeight <= 0 {
			t.Errorf("profile %s has invalid screen resolution: %dx%d", p.Name, p.ScreenWidth, p.ScreenHeight)
		}
	}
}
