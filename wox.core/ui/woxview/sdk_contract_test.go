package woxview

import (
	"os"
	"testing"
)

// TestSDKCounterContract checks that the shared SDK example is accepted by Core.
func TestSDKCounterContract(t *testing.T) {
	data, err := os.ReadFile("testdata/sdk-counter.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err != nil {
		t.Fatalf("parse SDK counter fixture: %v", err)
	}
}
