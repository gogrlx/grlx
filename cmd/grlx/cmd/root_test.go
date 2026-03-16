package cmd

import "testing"

// TestPackageInit verifies that importing this package does not panic
// when stdin is not a terminal (e.g. in test and CI environments).
// Previously, init() called UserConfirmWithDefault which panicked on EOF.
func TestPackageInit(t *testing.T) {
	// If we reach this point, init() completed without panic.
	if rootCmd == nil {
		t.Fatal("expected rootCmd to be initialized")
	}
}
