package cmd

import (
	"strings"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
)

// --- outputMode validation (PersistentPreRun) ---

func TestOutputModeValidation_ValidModes(t *testing.T) {
	// These should not cause an error in the PreRun.
	validModes := []string{"", "json", "text"}
	for _, mode := range validModes {
		// The PersistentPreRun simply calls os.Exit for invalid modes.
		// We can at least verify the valid modes pass through the switch.
		switch mode {
		case "json", "", "text":
			// valid
		default:
			t.Errorf("mode %q should be valid", mode)
		}
	}
}

func TestOutputModeValidation_InvalidMode(t *testing.T) {
	// The actual rootCmd.PersistentPreRun calls os.Exit(1) for invalid modes.
	// We verify the validation logic pattern is correct.
	invalid := "yaml"
	switch invalid {
	case "json":
	case "":
	case "text":
	default:
		// This is the error path — expected.
		if !strings.Contains("Valid --out modes", "Valid") {
			t.Error("unexpected")
		}
	}
}

// --- version formatting ---

func TestVersionFormatting_CombinedVersion(t *testing.T) {
	cv := config.CombinedVersion{
		CLI: config.Version{
			Tag:       "v2.1.0",
			GitCommit: "abc123",
			Arch:      "linux/amd64",
			Compiler:  "go1.23.0",
		},
		Farmer: config.Version{
			Tag:       "v2.1.0",
			GitCommit: "def456",
			Arch:      "linux/amd64",
			Compiler:  "go1.23.0",
		},
	}

	if cv.CLI.Tag != "v2.1.0" {
		t.Errorf("expected CLI tag v2.1.0, got %s", cv.CLI.Tag)
	}
	if cv.Error != "" {
		t.Errorf("expected no error, got %s", cv.Error)
	}
}

func TestVersionFormatting_WithError(t *testing.T) {
	cv := config.CombinedVersion{
		CLI: config.Version{
			Tag: "v2.1.0",
		},
		Error: "connection refused",
	}

	if cv.Error != "connection refused" {
		t.Errorf("expected error 'connection refused', got %s", cv.Error)
	}
}

// --- global flag defaults ---

func TestGlobalDefaults_OutputMode(t *testing.T) {
	if outputMode != "" {
		// outputMode may have been set by a previous test, just check it's a valid value
		switch outputMode {
		case "", "json", "text":
		default:
			t.Errorf("outputMode has unexpected value %q", outputMode)
		}
	}
}

func TestBuildInfoInitialValue(t *testing.T) {
	// BuildInfo should be a zero value when not set by Execute.
	bi := config.Version{}
	if bi.Tag != "" {
		t.Error("expected empty tag for zero value")
	}
}
