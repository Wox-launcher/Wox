package util

import "testing"

func TestShouldSkipOnboardingForTestRequiresTestMode(t *testing.T) {
	t.Setenv(TestWoxDataDirEnv, "")
	t.Setenv(TestUserDataDirEnv, "")
	t.Setenv(TestServerPortEnv, "")
	t.Setenv(TestSkipOnboardingEnv, "true")
	if ShouldSkipOnboardingForTest() {
		t.Fatal("skip-onboarding flag should not affect a normal Wox process")
	}
	t.Setenv(TestWoxDataDirEnv, t.TempDir())
	if !ShouldSkipOnboardingForTest() {
		t.Fatal("skip-onboarding flag should apply in test mode")
	}
}
