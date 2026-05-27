package util

import (
	"os"
	"testing"
)

func TestGetTestServerPortOverride(t *testing.T) {
	orig := os.Getenv(TestServerPortEnv)
	defer os.Setenv(TestServerPortEnv, orig)

	tests := []struct {
		name      string
		envValue  string
		wantPort  int
		wantError bool
	}{
		{"valid low port", "1", 1, false},
		{"valid high port", "65535", 65535, false},
		{"invalid zero", "0", 0, true},
		{"invalid negative", "-1", 0, true},
		{"invalid too high", "65536", 0, true},
		{"invalid non-numeric", "abc", 0, true},
		{"invalid empty", "", 0, true},
		{"invalid whitespace", "  ", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(TestServerPortEnv, tt.envValue)
			port, err := GetTestServerPortOverride()
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got port %d", port)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if port != tt.wantPort {
				t.Fatalf("expected port %d, got %d", tt.wantPort, port)
			}
		})
	}
}

func TestIsTestMode(t *testing.T) {
	origData := os.Getenv(TestWoxDataDirEnv)
	origUser := os.Getenv(TestUserDataDirEnv)
	origPort := os.Getenv(TestServerPortEnv)
	defer func() {
		os.Setenv(TestWoxDataDirEnv, origData)
		os.Setenv(TestUserDataDirEnv, origUser)
		os.Setenv(TestServerPortEnv, origPort)
	}()

	os.Unsetenv(TestWoxDataDirEnv)
	os.Unsetenv(TestUserDataDirEnv)
	os.Unsetenv(TestServerPortEnv)

	if IsTestMode() {
		t.Fatal("expected IsTestMode() to be false when no env vars are set")
	}

	os.Setenv(TestWoxDataDirEnv, "   ")
	os.Setenv(TestUserDataDirEnv, "   ")
	os.Setenv(TestServerPortEnv, "   ")
	if IsTestMode() {
		t.Fatal("expected IsTestMode() to be false when env vars are whitespace-only")
	}

	os.Setenv(TestServerPortEnv, "8080")
	if !IsTestMode() {
		t.Fatal("expected IsTestMode() to be true when port env is set")
	}
}

func TestShouldDisableTelemetryForTest(t *testing.T) {
	origData := os.Getenv(TestWoxDataDirEnv)
	origDisable := os.Getenv(TestDisableTelemetryEnv)
	defer func() {
		os.Setenv(TestWoxDataDirEnv, origData)
		os.Setenv(TestDisableTelemetryEnv, origDisable)
	}()

	os.Unsetenv(TestWoxDataDirEnv)
	os.Unsetenv(TestDisableTelemetryEnv)

	if ShouldDisableTelemetryForTest() {
		t.Fatal("expected ShouldDisableTelemetryForTest() to be false when not in test mode")
	}

	os.Setenv(TestWoxDataDirEnv, "/tmp/wox-test")

	os.Setenv(TestDisableTelemetryEnv, "false")
	if ShouldDisableTelemetryForTest() {
		t.Fatal("expected ShouldDisableTelemetryForTest() to be false when env is false")
	}

	os.Setenv(TestDisableTelemetryEnv, "TRUE")
	if !ShouldDisableTelemetryForTest() {
		t.Fatal("expected ShouldDisableTelemetryForTest() to be true when env is TRUE")
	}
}
