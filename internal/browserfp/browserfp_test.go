package browserfp

import (
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGenerateProducesCoherentProfile(t *testing.T) {
	seenPlatforms := make(map[string]bool)

	for seed := int64(0); seed < 500; seed++ {
		profile := Generate(rand.New(rand.NewSource(seed)))
		if profile == nil {
			t.Fatal("Generate returned nil")
		}
		seenPlatforms[profile.Platform] = true

		if profile.BuildID != DefaultBuildID {
			t.Fatalf("BuildID = %q, want %q", profile.BuildID, DefaultBuildID)
		}
		if profile.UserAgent == "" || !strings.Contains(profile.UserAgent, "Chrome/150.") {
			t.Fatalf("UserAgent = %q, want Chrome 150", profile.UserAgent)
		}
		if !userAgentMatchesPlatform(profile.UserAgent, profile.Platform) {
			t.Fatalf("UserAgent %q does not match platform %q", profile.UserAgent, profile.Platform)
		}
		if got := VendorForPlatform(profile.Platform); got != "Google Inc." {
			t.Fatalf("navigator.vendor = %q, want Google Inc.", got)
		}
		if !webGLMatchesPlatform(profile.Platform, profile.WebGLUnmaskedRenderer) {
			t.Fatalf("renderer %q does not match platform %q", profile.WebGLUnmaskedRenderer, profile.Platform)
		}
		if profile.WebGLUnmaskedVendor == "" || profile.WebGLUnmaskedRenderer == "" {
			t.Fatal("WebGL vendor or renderer is empty")
		}
		if profile.ScreenWidth <= 0 || profile.ScreenHeight <= 0 {
			t.Fatalf("invalid screen size %dx%d", profile.ScreenWidth, profile.ScreenHeight)
		}
		if profile.ScreenAvailHeight <= 0 || profile.ScreenAvailHeight >= profile.ScreenHeight {
			t.Fatalf("invalid available height %d for screen height %d", profile.ScreenAvailHeight, profile.ScreenHeight)
		}
		if profile.DevicePixelRatio <= 0 {
			t.Fatalf("invalid device pixel ratio %v", profile.DevicePixelRatio)
		}
		if profile.HardwareConcurrency <= 0 || profile.DeviceMemory <= 0 || profile.JSHeapSizeLimit <= 0 {
			t.Fatalf("invalid hardware values: cores=%d memory=%d heap=%d", profile.HardwareConcurrency, profile.DeviceMemory, profile.JSHeapSizeLimit)
		}
		if profile.NetworkDownlink <= 0 || profile.NetworkRTT < 50 || profile.NetworkRTT > 1000 {
			t.Fatalf("invalid network values: downlink=%v rtt=%d", profile.NetworkDownlink, profile.NetworkRTT)
		}
		if profile.Timezone == "" {
			t.Fatal("Timezone is empty")
		}
	}

	if len(seenPlatforms) != len(deviceProfiles) {
		t.Fatalf("generated %d platforms, want %d: %v", len(seenPlatforms), len(deviceProfiles), seenPlatforms)
	}
}

func TestGenerateIsDeterministicWithInjectedRand(t *testing.T) {
	now := time.Date(2026, time.August, 23, 12, 0, 0, 0, time.UTC)
	first := generateAt(rand.New(rand.NewSource(42)), now)
	second := generateAt(rand.New(rand.NewSource(42)), now)
	if *first != *second {
		t.Fatalf("Generate with equal seeds differs:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestGetReturnsStableCopies(t *testing.T) {
	Init()
	first := Get()
	second := Get()
	if first == nil || second == nil {
		t.Fatal("Get returned nil")
	}
	if first == second {
		t.Fatal("Get returned the shared internal pointer")
	}
	if *first != *second {
		t.Fatalf("Get returned different profiles: first=%#v second=%#v", first, second)
	}

	originalLanguage := second.Language
	first.Language = "mutated"
	if got := Get().Language; got != originalLanguage {
		t.Fatalf("mutating returned profile changed global state: got %q want %q", got, originalLanguage)
	}
}

func TestGetConcurrent(t *testing.T) {
	const goroutines = 64
	var wg sync.WaitGroup
	results := make(chan *Profile, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- Get()
		}()
	}
	wg.Wait()
	close(results)

	var want *Profile
	for profile := range results {
		if profile == nil {
			t.Fatal("Get returned nil concurrently")
		}
		if want == nil {
			want = profile
			continue
		}
		if *profile != *want {
			t.Fatalf("concurrent Get returned inconsistent profile: got=%#v want=%#v", profile, want)
		}
	}
}

func TestTimezoneOffsetMinutesHandlesDST(t *testing.T) {
	winter := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)
	summer := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	if got := timezoneOffsetMinutes("America/Los_Angeles", winter); got != 480 {
		t.Fatalf("winter offset = %d, want 480", got)
	}
	if got := timezoneOffsetMinutes("America/Los_Angeles", summer); got != 420 {
		t.Fatalf("summer offset = %d, want 420", got)
	}
	if got := timezoneOffsetMinutes("invalid/timezone", winter); got != 480 {
		t.Fatalf("fallback offset = %d, want 480", got)
	}
}

func userAgentMatchesPlatform(userAgent, platform string) bool {
	switch platform {
	case "Win32":
		return strings.Contains(userAgent, "Windows NT")
	case "MacIntel":
		return strings.Contains(userAgent, "Macintosh")
	case "Linux x86_64":
		return strings.Contains(userAgent, "X11; Linux x86_64")
	default:
		return false
	}
}
