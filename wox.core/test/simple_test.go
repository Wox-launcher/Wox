package test

import (
	"testing"
	"time"
)

// TestSimpleCalculator tests basic calculator functionality with minimal setup
func TestSimpleCalculator(t *testing.T) {
	suite := NewTestSuite(t)

	// Simple test with longer timeout
	test := QueryTest{
		Name:           "Simple addition",
		Query:          "1+2",
		ExpectedTitle:  "3",
		ExpectedAction: "Copy",
		Timeout:        90 * time.Second, // Longer timeout for debugging
	}

	t.Logf("Starting simple calculator test...")
	success := suite.RunQueryTest(test)
	if !success {
		t.Errorf("Simple calculator test failed")
	} else {
		t.Logf("Simple calculator test passed!")
	}
}

// TestServiceInitialization tests if services are properly initialized
func TestServiceInitialization(t *testing.T) {
	suite := NewTestSuite(t)

	// Just test that we can create a test suite without errors
	if suite == nil {
		t.Errorf("Failed to create test suite")
	} else {
		t.Logf("Test suite created successfully")
	}
}
