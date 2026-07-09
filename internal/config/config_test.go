package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// 清除所有相关环境变量
	for _, key := range []string{"SERVER_HOST", "SERVER_PORT", "PORT", "STREAM_MODE", "MAX_CONTINUE_COUNT"} {
		os.Unsetenv(key)
	}

	cfg := Load()

	if cfg.ServerHost != "0.0.0.0" {
		t.Errorf("ServerHost = %q, want %q", cfg.ServerHost, "0.0.0.0")
	}
	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "8080")
	}
	if !cfg.StreamMode {
		t.Error("StreamMode = false, want true")
	}
	if cfg.MaxContinueCount != 3 {
		t.Errorf("MaxContinueCount = %d, want 3", cfg.MaxContinueCount)
	}
}

func TestLoadWithEnv(t *testing.T) {
	os.Setenv("SERVER_HOST", "127.0.0.1")
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("STREAM_MODE", "false")
	defer func() {
		os.Unsetenv("SERVER_HOST")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("STREAM_MODE")
	}()

	cfg := Load()

	if cfg.ServerHost != "127.0.0.1" {
		t.Errorf("ServerHost = %q, want %q", cfg.ServerHost, "127.0.0.1")
	}
	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "9090")
	}
	if cfg.StreamMode {
		t.Error("StreamMode = true, want false")
	}
}

func TestGetBoolEnvInvalid(t *testing.T) {
	os.Setenv("TEST_BOOL", "notabool")
	defer os.Unsetenv("TEST_BOOL")

	result := getBoolEnv("TEST_BOOL", true)
	if !result {
		t.Error("getBoolEnv with invalid value should return default (true)")
	}
}
