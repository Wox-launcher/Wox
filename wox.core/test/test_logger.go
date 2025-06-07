package test

import (
	"context"
	"fmt"
	"wox/util"
)

// TestLogger wraps the original logger to use test-specific directories
type TestLogger struct {
	*util.Log
	testLocation *TestLocation
}

// NewTestLogger creates a new test logger that writes to test directories
func NewTestLogger(testLocation *TestLocation) *TestLogger {
	// Create logger with test log directory
	logFolder := testLocation.GetLogDirectory()
	logger := util.CreateLogger(logFolder)
	
	return &TestLogger{
		Log:          logger,
		testLocation: testLocation,
	}
}

// Override logging methods to add test prefix
func (tl *TestLogger) Debug(ctx context.Context, msg string) {
	tl.Log.Debug(ctx, fmt.Sprintf("[TEST] %s", msg))
}

func (tl *TestLogger) Info(ctx context.Context, msg string) {
	tl.Log.Info(ctx, fmt.Sprintf("[TEST] %s", msg))
}

func (tl *TestLogger) Warn(ctx context.Context, msg string) {
	tl.Log.Warn(ctx, fmt.Sprintf("[TEST] %s", msg))
}

func (tl *TestLogger) Error(ctx context.Context, msg string) {
	tl.Log.Error(ctx, fmt.Sprintf("[TEST] %s", msg))
}

// GetLogDirectory returns the test log directory
func (tl *TestLogger) GetLogDirectory() string {
	return tl.testLocation.GetLogDirectory()
}
