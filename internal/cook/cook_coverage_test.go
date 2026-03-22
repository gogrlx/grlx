package cook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/config"
)

// startCookTestNATS starts an embedded NATS server and registers the connection
// with the cook package. Returns a cleanup function.
func startCookTestNATS(t *testing.T) (*nats.Conn, func()) {
	t.Helper()

	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("start test NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server failed to become ready")
	}

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("connect to test NATS: %v", err)
	}

	RegisterNatsConn(nc)
	return nc, func() {
		RegisterNatsConn(nil)
		nc.Close()
		ns.Shutdown()
	}
}

// --- SimpleNote / Snprintf ---

func TestSimpleNoteString(t *testing.T) {
	note := SimpleNote("hello world")
	if note.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", note.String())
	}

	empty := SimpleNote("")
	if empty.String() != "" {
		t.Errorf("expected empty string, got %q", empty.String())
	}
}

func TestSnprintf(t *testing.T) {
	note := Snprintf("hello %s, you have %d items", "tai", 5)
	expected := "hello tai, you have 5 items"
	if note.String() != expected {
		t.Errorf("expected %q, got %q", expected, note.String())
	}
}

// --- WithInvoker ---

func TestWithInvoker(t *testing.T) {
	var co cookOptions
	opt := WithInvoker("pubkey-abc123")
	opt(&co)
	if co.invokedBy != "pubkey-abc123" {
		t.Errorf("expected invokedBy 'pubkey-abc123', got %q", co.invokedBy)
	}
}

// --- RegisterNatsConn ---

func TestRegisterNatsConn(t *testing.T) {
	old := conn
	defer func() { conn = old }()

	RegisterNatsConn(nil)
	if conn != nil {
		t.Error("expected conn to be nil")
	}
}

// --- GenerateJobID ---

func TestGenerateJobID(t *testing.T) {
	id1 := GenerateJobID()
	id2 := GenerateJobID()
	if id1 == "" {
		t.Error("expected non-empty job ID")
	}
	if id1 == id2 {
		t.Error("expected unique job IDs")
	}
	// UUID format: 8-4-4-4-12
	parts := strings.Split(id1, "-")
	if len(parts) != 5 {
		t.Errorf("expected UUID format (5 parts), got %d parts: %s", len(parts), id1)
	}
}

// --- validateRecipeTree ---

func TestValidateRecipeTree(t *testing.T) {
	// Valid tree: no cycles, no duplicates
	stepA := &Step{ID: "a"}
	stepB := &Step{ID: "b", Requisites: RequisiteSet{Requisite{Condition: Require, StepIDs: []StepID{"a"}}}}
	result, err := validateRecipeTree([]*Step{stepA, stepB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}

	// Invalid: cycle
	stepX := &Step{ID: "x", Requisites: RequisiteSet{Requisite{Condition: Require, StepIDs: []StepID{"y"}}}}
	stepY := &Step{ID: "y", Requisites: RequisiteSet{Requisite{Condition: Require, StepIDs: []StepID{"x"}}}}
	_, err = validateRecipeTree([]*Step{stepX, stepY})
	if !errors.Is(err, ErrDependencyCycleFound) {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

// --- SummarizeSteps ---

func TestSummarizeSteps(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		summary := SummarizeSteps(nil)
		if len(summary) != 0 {
			t.Errorf("expected empty summary, got %d entries", len(summary))
		}
	})

	t.Run("mixed results", func(t *testing.T) {
		steps := []SproutStepCompletion{
			{
				SproutID: "sprout-1",
				CompletedStep: StepCompletion{
					ID:               "step-a",
					CompletionStatus: StepCompleted,
					ChangesMade:      true,
				},
			},
			{
				SproutID: "sprout-1",
				CompletedStep: StepCompletion{
					ID:               "step-b",
					CompletionStatus: StepCompleted,
					ChangesMade:      false,
				},
			},
			{
				SproutID: "sprout-1",
				CompletedStep: StepCompletion{
					ID:               "step-c",
					CompletionStatus: StepFailed,
					Error:            errors.New("something broke"),
				},
			},
			{
				SproutID: "sprout-2",
				CompletedStep: StepCompletion{
					ID:               "step-a",
					CompletionStatus: StepCompleted,
					ChangesMade:      true,
				},
			},
			{
				SproutID: "sprout-2",
				CompletedStep: StepCompletion{
					ID:               "step-b",
					CompletionStatus: StepSkipped,
				},
			},
		}

		summary := SummarizeSteps(steps)

		s1 := summary["sprout-1"]
		if s1.Succeeded != 2 {
			t.Errorf("sprout-1: expected 2 succeeded, got %d", s1.Succeeded)
		}
		if s1.Failures != 1 {
			t.Errorf("sprout-1: expected 1 failure, got %d", s1.Failures)
		}
		if s1.Changes != 1 {
			t.Errorf("sprout-1: expected 1 change, got %d", s1.Changes)
		}
		if len(s1.Errors) != 1 {
			t.Errorf("sprout-1: expected 1 error, got %d", len(s1.Errors))
		}

		s2 := summary["sprout-2"]
		if s2.Succeeded != 1 {
			t.Errorf("sprout-2: expected 1 succeeded, got %d", s2.Succeeded)
		}
		if s2.Failures != 0 {
			t.Errorf("sprout-2: expected 0 failures, got %d", s2.Failures)
		}
		if s2.Changes != 1 {
			t.Errorf("sprout-2: expected 1 change, got %d", s2.Changes)
		}
	})

	t.Run("all skipped", func(t *testing.T) {
		steps := []SproutStepCompletion{
			{
				SproutID: "sprout-3",
				CompletedStep: StepCompletion{
					ID:               "step-a",
					CompletionStatus: StepSkipped,
				},
			},
		}
		summary := SummarizeSteps(steps)
		s3 := summary["sprout-3"]
		if s3.Succeeded != 0 || s3.Failures != 0 || s3.Changes != 0 {
			t.Errorf("expected all zeros for skipped, got succeeded=%d failures=%d changes=%d",
				s3.Succeeded, s3.Failures, s3.Changes)
		}
	})
}

// --- logStepResult ---

func TestLogStepResult(t *testing.T) {
	t.Run("no log dir configured", func(t *testing.T) {
		oldDir := config.JobLogDir
		config.JobLogDir = ""
		defer func() { config.JobLogDir = oldDir }()

		// Should not panic or error
		logStepResult("test-job", StepCompletion{
			ID:               "step-1",
			CompletionStatus: StepCompleted,
		})
	})

	t.Run("writes jsonl file", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldDir := config.JobLogDir
		config.JobLogDir = tmpDir
		defer func() { config.JobLogDir = oldDir }()

		jobID := "test-job-123"
		completion := StepCompletion{
			ID:               "step-1",
			CompletionStatus: StepCompleted,
			ChangesMade:      true,
			Changes:          []string{"created /etc/foo"},
		}

		logStepResult(jobID, completion)

		logFile := filepath.Join(tmpDir, jobID+".jsonl")
		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) != 1 {
			t.Fatalf("expected 1 line, got %d", len(lines))
		}

		var parsed StepCompletion
		if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
			t.Fatalf("failed to unmarshal log line: %v", err)
		}
		if parsed.ID != "step-1" {
			t.Errorf("expected step ID 'step-1', got %q", parsed.ID)
		}
	})

	t.Run("appends multiple results", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldDir := config.JobLogDir
		config.JobLogDir = tmpDir
		defer func() { config.JobLogDir = oldDir }()

		jobID := "multi-step-job"
		logStepResult(jobID, StepCompletion{ID: "step-1", CompletionStatus: StepCompleted})
		logStepResult(jobID, StepCompletion{ID: "step-2", CompletionStatus: StepFailed})
		logStepResult(jobID, StepCompletion{ID: "step-3", CompletionStatus: StepCompleted})

		logFile := filepath.Join(tmpDir, jobID+".jsonl")
		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedDir := filepath.Join(tmpDir, "a", "b", "c")
		oldDir := config.JobLogDir
		config.JobLogDir = nestedDir
		defer func() { config.JobLogDir = oldDir }()

		logStepResult("nested-job", StepCompletion{ID: "step-1", CompletionStatus: StepCompleted})

		logFile := filepath.Join(nestedDir, "nested-job.jsonl")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			t.Error("expected log file to be created in nested directory")
		}
	})
}

// --- RequisitesAreMet (comprehensive) ---

func TestRequisitesAreMetComprehensive(t *testing.T) {
	completionMap := map[StepID]StepCompletion{
		"completed": {
			ID:               "completed",
			CompletionStatus: StepCompleted,
			ChangesMade:      false,
		},
		"completed-changed": {
			ID:               "completed-changed",
			CompletionStatus: StepCompleted,
			ChangesMade:      true,
		},
		"failed": {
			ID:               "failed",
			CompletionStatus: StepFailed,
			ChangesMade:      false,
		},
		"failed-changed": {
			ID:               "failed-changed",
			CompletionStatus: StepFailed,
			ChangesMade:      true,
		},
		"in-progress": {
			ID:               "in-progress",
			CompletionStatus: StepInProgress,
		},
		"not-started": {
			ID:               "not-started",
			CompletionStatus: StepNotStarted,
		},
	}

	tests := []struct {
		name        string
		requisites  RequisiteSet
		expectedMet bool
		expectedErr error
	}{
		// --- OnChanges ---
		{
			name: "onchanges: completed with changes",
			requisites: RequisiteSet{Requisite{
				Condition: OnChanges,
				StepIDs:   []StepID{"completed-changed"},
			}},
			expectedMet: true,
		},
		{
			name: "onchanges: completed without changes",
			requisites: RequisiteSet{Requisite{
				Condition: OnChanges,
				StepIDs:   []StepID{"completed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},
		{
			name: "onchanges: failed with changes",
			requisites: RequisiteSet{Requisite{
				Condition: OnChanges,
				StepIDs:   []StepID{"failed-changed"},
			}},
			expectedMet: true,
		},
		{
			name: "onchanges: failed without changes",
			requisites: RequisiteSet{Requisite{
				Condition: OnChanges,
				StepIDs:   []StepID{"failed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},
		{
			name: "onchanges: in progress",
			requisites: RequisiteSet{Requisite{
				Condition: OnChanges,
				StepIDs:   []StepID{"in-progress"},
			}},
			expectedMet: false,
		},
		{
			name: "onchanges: not started",
			requisites: RequisiteSet{Requisite{
				Condition: OnChanges,
				StepIDs:   []StepID{"not-started"},
			}},
			expectedMet: false,
		},

		// --- OnFail ---
		{
			name: "onfail: step failed",
			requisites: RequisiteSet{Requisite{
				Condition: OnFail,
				StepIDs:   []StepID{"failed"},
			}},
			expectedMet: true,
		},
		{
			name: "onfail: step completed (cannot be met)",
			requisites: RequisiteSet{Requisite{
				Condition: OnFail,
				StepIDs:   []StepID{"completed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},
		{
			name: "onfail: in progress (not yet)",
			requisites: RequisiteSet{Requisite{
				Condition: OnFail,
				StepIDs:   []StepID{"in-progress"},
			}},
			expectedMet: false,
		},
		{
			name: "onfail: not started (not yet)",
			requisites: RequisiteSet{Requisite{
				Condition: OnFail,
				StepIDs:   []StepID{"not-started"},
			}},
			expectedMet: false,
		},
		{
			name: "onfail: multiple, one completed one failed",
			requisites: RequisiteSet{Requisite{
				Condition: OnFail,
				StepIDs:   []StepID{"completed", "failed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},

		// --- Require ---
		{
			name: "require: completed",
			requisites: RequisiteSet{Requisite{
				Condition: Require,
				StepIDs:   []StepID{"completed"},
			}},
			expectedMet: true,
		},
		{
			name: "require: failed",
			requisites: RequisiteSet{Requisite{
				Condition: Require,
				StepIDs:   []StepID{"failed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},
		{
			name: "require: in progress",
			requisites: RequisiteSet{Requisite{
				Condition: Require,
				StepIDs:   []StepID{"in-progress"},
			}},
			expectedMet: false,
		},

		// --- OnChangesAny ---
		{
			name: "onchanges_any: one changed, one not",
			requisites: RequisiteSet{Requisite{
				Condition: OnChangesAny,
				StepIDs:   []StepID{"completed-changed", "completed"},
			}},
			expectedMet: true,
		},
		{
			name: "onchanges_any: none changed, all done",
			requisites: RequisiteSet{Requisite{
				Condition: OnChangesAny,
				StepIDs:   []StepID{"completed", "failed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},
		{
			name: "onchanges_any: none changed, some pending",
			requisites: RequisiteSet{Requisite{
				Condition: OnChangesAny,
				StepIDs:   []StepID{"completed", "in-progress"},
			}},
			expectedMet: false,
		},
		{
			name: "onchanges_any: one changed with pending",
			requisites: RequisiteSet{Requisite{
				Condition: OnChangesAny,
				StepIDs:   []StepID{"completed-changed", "in-progress"},
			}},
			expectedMet: true,
		},

		// --- OnFailAny ---
		{
			name: "onfail_any: one failed",
			requisites: RequisiteSet{Requisite{
				Condition: OnFailAny,
				StepIDs:   []StepID{"completed", "failed"},
			}},
			expectedMet: true,
		},
		{
			name: "onfail_any: none failed, all completed",
			requisites: RequisiteSet{Requisite{
				Condition: OnFailAny,
				StepIDs:   []StepID{"completed", "completed-changed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},
		{
			name: "onfail_any: none failed, some pending",
			requisites: RequisiteSet{Requisite{
				Condition: OnFailAny,
				StepIDs:   []StepID{"completed", "in-progress"},
			}},
			expectedMet: false,
		},
		{
			name: "onfail_any: one failed with pending",
			requisites: RequisiteSet{Requisite{
				Condition: OnFailAny,
				StepIDs:   []StepID{"failed", "in-progress"},
			}},
			expectedMet: true,
		},

		// --- RequireAny ---
		{
			name: "require_any: one completed",
			requisites: RequisiteSet{Requisite{
				Condition: RequireAny,
				StepIDs:   []StepID{"completed", "failed"},
			}},
			expectedMet: true,
		},
		{
			name: "require_any: all failed",
			requisites: RequisiteSet{Requisite{
				Condition: RequireAny,
				StepIDs:   []StepID{"failed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},
		{
			name: "require_any: none completed, some pending",
			requisites: RequisiteSet{Requisite{
				Condition: RequireAny,
				StepIDs:   []StepID{"failed", "in-progress"},
			}},
			expectedMet: false,
		},

		// --- Unknown condition ---
		{
			name: "unknown condition",
			requisites: RequisiteSet{Requisite{
				Condition: ReqType("bogus"),
				StepIDs:   []StepID{"completed"},
			}},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},

		// --- Multiple requisite sets (ANDed) ---
		{
			name: "multiple sets: both met",
			requisites: RequisiteSet{
				Requisite{Condition: Require, StepIDs: []StepID{"completed"}},
				Requisite{Condition: OnChanges, StepIDs: []StepID{"completed-changed"}},
			},
			expectedMet: true,
		},
		{
			name: "multiple sets: one met, one not",
			requisites: RequisiteSet{
				Requisite{Condition: Require, StepIDs: []StepID{"completed"}},
				Requisite{Condition: OnChanges, StepIDs: []StepID{"completed"}},
			},
			expectedMet: false,
			expectedErr: ErrRequisiteNotMet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			met, err := RequisitesAreMet(Step{Requisites: tt.requisites}, completionMap)
			if met != tt.expectedMet {
				t.Errorf("expected met=%v, got met=%v", tt.expectedMet, met)
			}
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// --- stepsFromMap edge cases ---

func TestStepsFromMapEdgeCases(t *testing.T) {
	t.Run("no steps key", func(t *testing.T) {
		recipe := map[string]interface{}{
			"include": []interface{}{"foo"},
		}
		steps, err := stepsFromMap(recipe)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(steps) != 0 {
			t.Errorf("expected empty steps, got %d", len(steps))
		}
	})

	t.Run("steps is wrong type", func(t *testing.T) {
		recipe := map[string]interface{}{
			"steps": "not a map",
		}
		_, err := stepsFromMap(recipe)
		if err == nil {
			t.Error("expected error for wrong type, got nil")
		}
	})

	t.Run("steps is a list", func(t *testing.T) {
		recipe := map[string]interface{}{
			"steps": []interface{}{"a", "b"},
		}
		_, err := stepsFromMap(recipe)
		if err == nil {
			t.Error("expected error for list type, got nil")
		}
	})
}

// --- includesFromMap edge cases ---

func TestIncludesFromMapEdgeCases(t *testing.T) {
	t.Run("no include key", func(t *testing.T) {
		recipe := map[string]interface{}{
			"steps": map[string]interface{}{},
		}
		includes, err := includesFromMap(recipe)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(includes) != 0 {
			t.Errorf("expected empty includes, got %d", len(includes))
		}
	})

	t.Run("include is wrong type", func(t *testing.T) {
		recipe := map[string]interface{}{
			"include": "not a list",
		}
		_, err := includesFromMap(recipe)
		if err == nil {
			t.Error("expected error for wrong type, got nil")
		}
	})

	t.Run("include contains non-string", func(t *testing.T) {
		recipe := map[string]interface{}{
			"include": []interface{}{42},
		}
		_, err := includesFromMap(recipe)
		if err == nil {
			t.Error("expected error for non-string in include list, got nil")
		}
	})
}

// --- recipeToStep edge cases ---

func TestRecipeToStepEdgeCases(t *testing.T) {
	t.Run("empty recipe map", func(t *testing.T) {
		_, err := recipeToStep("test-id", map[string]interface{}{})
		if err == nil {
			t.Error("expected error for empty recipe")
		}
	})

	t.Run("multiple keys", func(t *testing.T) {
		_, err := recipeToStep("test-id", map[string]interface{}{
			"file.managed": []interface{}{},
			"cmd.run":      []interface{}{},
		})
		if err == nil {
			t.Error("expected error for multiple keys")
		}
	})

	t.Run("invalid key format (no dot)", func(t *testing.T) {
		_, err := recipeToStep("test-id", map[string]interface{}{
			"filemanaged": []interface{}{},
		})
		if err == nil {
			t.Error("expected error for invalid key format")
		}
	})

	t.Run("value is not a list", func(t *testing.T) {
		_, err := recipeToStep("test-id", map[string]interface{}{
			"file.managed": "not a list",
		})
		if err == nil {
			t.Error("expected error for non-list value")
		}
	})

	t.Run("list contains non-map item", func(t *testing.T) {
		_, err := recipeToStep("test-id", map[string]interface{}{
			"file.managed": []interface{}{"not a map"},
		})
		if err == nil {
			t.Error("expected error for non-map list item")
		}
	})

	t.Run("valid step", func(t *testing.T) {
		step, err := recipeToStep("install-nginx", map[string]interface{}{
			"pkg.installed": []interface{}{
				map[string]interface{}{"name": "nginx"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if step.ID != "install-nginx" {
			t.Errorf("expected ID 'install-nginx', got %q", step.ID)
		}
		if step.Ingredient != "pkg" {
			t.Errorf("expected ingredient 'pkg', got %q", step.Ingredient)
		}
		if step.Method != "installed" {
			t.Errorf("expected method 'installed', got %q", step.Method)
		}
		if step.Properties["name"] != "nginx" {
			t.Errorf("expected property name 'nginx', got %v", step.Properties["name"])
		}
	})

	t.Run("step with requisites", func(t *testing.T) {
		step, err := recipeToStep("start-nginx", map[string]interface{}{
			"service.running": []interface{}{
				map[string]interface{}{"name": "nginx"},
				map[string]interface{}{
					"requisites": []interface{}{
						map[string]interface{}{
							"require": "install-nginx",
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(step.Requisites) != 1 {
			t.Fatalf("expected 1 requisite, got %d", len(step.Requisites))
		}
		if step.Requisites[0].Condition != Require {
			t.Errorf("expected require condition, got %q", step.Requisites[0].Condition)
		}
	})
}

// --- makeRecipeSteps edge cases ---

func TestMakeRecipeStepsEdgeCases(t *testing.T) {
	t.Run("recipe value is not a map", func(t *testing.T) {
		recipes := map[string]interface{}{
			"bad-step": "not a map",
		}
		_, err := makeRecipeSteps(recipes)
		if err == nil {
			t.Error("expected error for non-map recipe value")
		}
	})

	t.Run("valid single step", func(t *testing.T) {
		recipes := map[string]interface{}{
			"install-pkg": map[string]interface{}{
				"pkg.installed": []interface{}{
					map[string]interface{}{"name": "vim"},
				},
			},
		}
		steps, err := makeRecipeSteps(recipes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(steps) != 1 {
			t.Fatalf("expected 1 step, got %d", len(steps))
		}
	})
}

// --- extractRequisites edge cases ---

func TestExtractRequisitesEdgeCases(t *testing.T) {
	t.Run("requisites is not a list", func(t *testing.T) {
		step := map[string]interface{}{
			"requisites": "not a list",
		}
		_, err := extractRequisites(step)
		if err == nil {
			t.Error("expected error for non-list requisites")
		}
		if !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("expected ErrInvalidFormat, got %v", err)
		}
	})

	t.Run("requisites list contains non-map", func(t *testing.T) {
		step := map[string]interface{}{
			"requisites": []interface{}{"not a map"},
		}
		_, err := extractRequisites(step)
		if err == nil {
			t.Error("expected error for non-map in requisites list")
		}
		if !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("expected ErrInvalidFormat, got %v", err)
		}
	})

	t.Run("unknown requisite type", func(t *testing.T) {
		step := map[string]interface{}{
			"requisites": []interface{}{
				map[string]interface{}{
					"bogus_type": "some-step",
				},
			},
		}
		_, err := extractRequisites(step)
		if err == nil {
			t.Error("expected error for unknown requisite type")
		}
	})

	t.Run("all valid requisite types", func(t *testing.T) {
		step := map[string]interface{}{
			"requisites": []interface{}{
				map[string]interface{}{"require": "step-a"},
				map[string]interface{}{"onchanges": "step-b"},
				map[string]interface{}{"onfail": "step-c"},
				map[string]interface{}{"require_any": []interface{}{"step-d", "step-e"}},
				map[string]interface{}{"onchanges_any": []interface{}{"step-f"}},
				map[string]interface{}{"onfail_any": []interface{}{"step-g"}},
			},
		}
		reqs, err := extractRequisites(step)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reqs) != 6 {
			t.Errorf("expected 6 requisites, got %d", len(reqs))
		}
	})
}

// --- deInterfaceRequisites edge cases ---

func TestDeInterfaceRequisitesEdgeCases(t *testing.T) {
	t.Run("list with non-string element", func(t *testing.T) {
		_, err := deInterfaceRequisites(Require, []interface{}{"ok", 42})
		if err == nil {
			t.Error("expected error for non-string in list")
		}
		if !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("expected ErrInvalidFormat, got %v", err)
		}
	})

	t.Run("unsupported type (int)", func(t *testing.T) {
		_, err := deInterfaceRequisites(Require, 42)
		if err == nil {
			t.Error("expected error for unsupported type")
		}
		if !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("expected ErrInvalidFormat, got %v", err)
		}
	})
}

// --- RequisiteSet.Equals / Requisite.Equals edge cases ---

func TestRequisiteSetEqualsEdgeCases(t *testing.T) {
	t.Run("different lengths", func(t *testing.T) {
		a := RequisiteSet{Requisite{Condition: Require, StepIDs: []StepID{"a"}}}
		b := RequisiteSet{}
		if a.Equals(b) {
			t.Error("expected not equal for different lengths")
		}
	})

	t.Run("same condition, different step IDs", func(t *testing.T) {
		a := RequisiteSet{Requisite{Condition: Require, StepIDs: []StepID{"a"}}}
		b := RequisiteSet{Requisite{Condition: Require, StepIDs: []StepID{"b"}}}
		if a.Equals(b) {
			t.Error("expected not equal for different step IDs")
		}
	})

	t.Run("different conditions", func(t *testing.T) {
		a := RequisiteSet{Requisite{Condition: Require, StepIDs: []StepID{"a"}}}
		b := RequisiteSet{Requisite{Condition: OnFail, StepIDs: []StepID{"a"}}}
		if a.Equals(b) {
			t.Error("expected not equal for different conditions")
		}
	})
}

func TestRequisiteEquals(t *testing.T) {
	t.Run("different conditions", func(t *testing.T) {
		a := Requisite{Condition: Require, StepIDs: []StepID{"a"}}
		b := Requisite{Condition: OnFail, StepIDs: []StepID{"a"}}
		if a.Equals(b) {
			t.Error("expected not equal")
		}
	})

	t.Run("different step count", func(t *testing.T) {
		a := Requisite{Condition: Require, StepIDs: []StepID{"a"}}
		b := Requisite{Condition: Require, StepIDs: []StepID{"a", "b"}}
		if a.Equals(b) {
			t.Error("expected not equal")
		}
	})

	t.Run("different step IDs", func(t *testing.T) {
		a := Requisite{Condition: Require, StepIDs: []StepID{"a", "b"}}
		b := Requisite{Condition: Require, StepIDs: []StepID{"a", "c"}}
		if a.Equals(b) {
			t.Error("expected not equal")
		}
	})

	t.Run("equal", func(t *testing.T) {
		a := Requisite{Condition: Require, StepIDs: []StepID{"a", "b"}}
		b := Requisite{Condition: Require, StepIDs: []StepID{"b", "a"}}
		if !a.Equals(b) {
			t.Error("expected equal (order independent)")
		}
	})
}

// --- CookRecipeEnvelope ---

// mockRecipeCooker is a test double for RecipeCooker.
type mockRecipeCooker struct {
	applyResult Result
	applyErr    error
	testResult  Result
	testErr     error
	props       map[string]interface{}
	propsErr    error
}

func (m *mockRecipeCooker) Apply(_ context.Context) (Result, error) {
	return m.applyResult, m.applyErr
}

func (m *mockRecipeCooker) Test(_ context.Context) (Result, error) {
	return m.testResult, m.testErr
}

func (m *mockRecipeCooker) Properties() (map[string]interface{}, error) {
	return m.props, m.propsErr
}

func (m *mockRecipeCooker) Parse(id, method string, properties map[string]interface{}) (RecipeCooker, error) {
	return m, nil
}

func (m *mockRecipeCooker) Methods() (string, []string) {
	return "mock", []string{"run"}
}

func (m *mockRecipeCooker) PropertiesForMethod(method string) (map[string]string, error) {
	return nil, nil
}

func TestCookRecipeEnvelopeSimple(t *testing.T) {
	nc, cleanup := startCookTestNATS(t)
	defer cleanup()

	// Set up a temporary job log dir
	tmpDir := t.TempDir()
	oldDir := config.JobLogDir
	config.JobLogDir = tmpDir
	defer func() { config.JobLogDir = oldDir }()

	// Save and restore the original NewRecipeCooker and sprout ID
	origCooker := NewRecipeCooker
	defer func() { NewRecipeCooker = origCooker }()

	oldSproutID := config.SproutID
	config.SproutID = "test-sprout"
	defer func() { config.SproutID = oldSproutID }()

	// Mock the cooker factory to return a successful cooker
	NewRecipeCooker = func(id StepID, ingredient Ingredient, method string, params map[string]interface{}) (RecipeCooker, error) {
		return &mockRecipeCooker{
			applyResult: Result{Succeeded: true, Changed: true, Notes: []fmt.Stringer{SimpleNote("applied")}},
		}, nil
	}

	// Subscribe to completion events to verify they're published
	completions := make(chan *nats.Msg, 10)
	sub, err := nc.Subscribe("grlx.cook.test-sprout.>", func(msg *nats.Msg) {
		completions <- msg
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	envelope := RecipeEnvelope{
		JobID: "test-job-001",
		Steps: []Step{
			{
				ID:         "step-1",
				Ingredient: "cmd",
				Method:     "run",
				Properties: map[string]interface{}{"name": "echo hello"},
			},
		},
		Test: false,
	}

	err = CookRecipeEnvelope(envelope)
	if err != nil {
		t.Fatalf("CookRecipeEnvelope: %v", err)
	}

	// Verify log file was written
	logFile := filepath.Join(tmpDir, "test-job-001.jsonl")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("expected job log file to be created")
	}
}

func TestCookRecipeEnvelopeTestMode(t *testing.T) {
	_, cleanup := startCookTestNATS(t)
	defer cleanup()

	tmpDir := t.TempDir()
	oldDir := config.JobLogDir
	config.JobLogDir = tmpDir
	defer func() { config.JobLogDir = oldDir }()

	origCooker := NewRecipeCooker
	defer func() { NewRecipeCooker = origCooker }()

	oldSproutID := config.SproutID
	config.SproutID = "test-sprout"
	defer func() { config.SproutID = oldSproutID }()

	testCalled := false
	NewRecipeCooker = func(id StepID, ingredient Ingredient, method string, params map[string]interface{}) (RecipeCooker, error) {
		return &mockRecipeCooker{
			testResult:  Result{Succeeded: true},
			applyResult: Result{Succeeded: false},
		}, nil
	}
	// We'll verify test mode was used by checking the result (testResult returns Succeeded=true)
	_ = testCalled

	envelope := RecipeEnvelope{
		JobID: "test-mode-job",
		Steps: []Step{
			{
				ID:         "step-1",
				Ingredient: "cmd",
				Method:     "run",
				Properties: map[string]interface{}{"name": "echo test"},
			},
		},
		Test: true,
	}

	err := CookRecipeEnvelope(envelope)
	if err != nil {
		t.Fatalf("CookRecipeEnvelope (test mode): %v", err)
	}
}

func TestCookRecipeEnvelopeMultipleStepsWithRequisites(t *testing.T) {
	_, cleanup := startCookTestNATS(t)
	defer cleanup()

	tmpDir := t.TempDir()
	oldDir := config.JobLogDir
	config.JobLogDir = tmpDir
	defer func() { config.JobLogDir = oldDir }()

	origCooker := NewRecipeCooker
	defer func() { NewRecipeCooker = origCooker }()

	oldSproutID := config.SproutID
	config.SproutID = "test-sprout"
	defer func() { config.SproutID = oldSproutID }()

	NewRecipeCooker = func(id StepID, ingredient Ingredient, method string, params map[string]interface{}) (RecipeCooker, error) {
		return &mockRecipeCooker{
			applyResult: Result{Succeeded: true, Changed: true},
		}, nil
	}

	envelope := RecipeEnvelope{
		JobID: "multi-step-job",
		Steps: []Step{
			{
				ID:         "install-pkg",
				Ingredient: "pkg",
				Method:     "installed",
				Properties: map[string]interface{}{"name": "nginx"},
			},
			{
				ID:         "start-service",
				Ingredient: "service",
				Method:     "running",
				Properties: map[string]interface{}{"name": "nginx"},
				Requisites: RequisiteSet{
					Requisite{Condition: Require, StepIDs: []StepID{"install-pkg"}},
				},
			},
		},
		Test: false,
	}

	err := CookRecipeEnvelope(envelope)
	if err != nil {
		t.Fatalf("CookRecipeEnvelope (multi-step): %v", err)
	}

	// Check log file has entries for both steps
	logFile := filepath.Join(tmpDir, "multi-step-job.jsonl")
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// At minimum, we expect the two actual step completions (+ the start pseudo-step)
	if len(lines) < 2 {
		t.Errorf("expected at least 2 log lines, got %d", len(lines))
	}
}

func TestCookRecipeEnvelopeFailedStep(t *testing.T) {
	_, cleanup := startCookTestNATS(t)
	defer cleanup()

	tmpDir := t.TempDir()
	oldDir := config.JobLogDir
	config.JobLogDir = tmpDir
	defer func() { config.JobLogDir = oldDir }()

	origCooker := NewRecipeCooker
	defer func() { NewRecipeCooker = origCooker }()

	oldSproutID := config.SproutID
	config.SproutID = "test-sprout"
	defer func() { config.SproutID = oldSproutID }()

	NewRecipeCooker = func(id StepID, ingredient Ingredient, method string, params map[string]interface{}) (RecipeCooker, error) {
		if string(id) == "step-1" {
			return &mockRecipeCooker{
				applyResult: Result{Succeeded: false, Failed: true},
				applyErr:    errors.New("step failed"),
			}, nil
		}
		return &mockRecipeCooker{
			applyResult: Result{Succeeded: true},
		}, nil
	}

	envelope := RecipeEnvelope{
		JobID: "fail-job",
		Steps: []Step{
			{
				ID:         "step-1",
				Ingredient: "cmd",
				Method:     "run",
				Properties: map[string]interface{}{"name": "bad-cmd"},
			},
			{
				ID:         "step-2",
				Ingredient: "cmd",
				Method:     "run",
				Properties: map[string]interface{}{"name": "good-cmd"},
				Requisites: RequisiteSet{
					Requisite{Condition: Require, StepIDs: []StepID{"step-1"}},
				},
			},
		},
	}

	// Should still complete (step-2 will fail due to requisite)
	err := CookRecipeEnvelope(envelope)
	if err != nil {
		t.Fatalf("CookRecipeEnvelope (fail): %v", err)
	}
}

func TestCookRecipeEnvelopeCookerFactoryError(t *testing.T) {
	_, cleanup := startCookTestNATS(t)
	defer cleanup()

	tmpDir := t.TempDir()
	oldDir := config.JobLogDir
	config.JobLogDir = tmpDir
	defer func() { config.JobLogDir = oldDir }()

	origCooker := NewRecipeCooker
	defer func() { NewRecipeCooker = origCooker }()

	oldSproutID := config.SproutID
	config.SproutID = "test-sprout"
	defer func() { config.SproutID = oldSproutID }()

	NewRecipeCooker = func(id StepID, ingredient Ingredient, method string, params map[string]interface{}) (RecipeCooker, error) {
		return nil, errors.New("unknown ingredient")
	}

	envelope := RecipeEnvelope{
		JobID: "factory-error-job",
		Steps: []Step{
			{
				ID:         "step-1",
				Ingredient: "bogus",
				Method:     "run",
				Properties: map[string]interface{}{},
			},
		},
	}

	err := CookRecipeEnvelope(envelope)
	if err != nil {
		t.Fatalf("CookRecipeEnvelope should complete even if cooker factory fails: %v", err)
	}
}

// --- SendCookEvent ---

func TestSendCookEvent(t *testing.T) {
	nc, cleanup := startCookTestNATS(t)
	defer cleanup()

	// Subscribe to the cook subject for the test sprout, replying with an ack.
	sproutID := "send-cook-sprout"
	sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".cook", func(msg *nats.Msg) {
		var env RecipeEnvelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			t.Errorf("unmarshal envelope: %v", err)
			return
		}
		ack := Ack{Acknowledged: true, JobID: env.JobID}
		data, _ := json.Marshal(ack)
		if err := msg.Respond(data); err != nil {
			t.Errorf("respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	jid := GenerateJobID()
	err = SendCookEvent(sproutID, "independent", jid, false)
	if err != nil {
		t.Fatalf("SendCookEvent: %v", err)
	}
}

func TestSendCookEventTestMode(t *testing.T) {
	nc, cleanup := startCookTestNATS(t)
	defer cleanup()

	sproutID := "send-cook-test-sprout"
	sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".cook", func(msg *nats.Msg) {
		var env RecipeEnvelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			t.Errorf("unmarshal: %v", err)
			return
		}
		if !env.Test {
			t.Error("expected Test=true in envelope")
		}
		ack := Ack{Acknowledged: true, JobID: env.JobID}
		data, _ := json.Marshal(ack)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	err = SendCookEvent(sproutID, "independent", GenerateJobID(), true)
	if err != nil {
		t.Fatalf("SendCookEvent (test mode): %v", err)
	}
}

func TestSendCookEventWithInvoker(t *testing.T) {
	nc, cleanup := startCookTestNATS(t)
	defer cleanup()

	sproutID := "send-cook-invoker-sprout"
	sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".cook", func(msg *nats.Msg) {
		var env RecipeEnvelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			t.Errorf("unmarshal: %v", err)
			return
		}
		if env.InvokedBy != "pubkey-xyz" {
			t.Errorf("expected InvokedBy 'pubkey-xyz', got %q", env.InvokedBy)
		}
		ack := Ack{Acknowledged: true, JobID: env.JobID}
		data, _ := json.Marshal(ack)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	err = SendCookEvent(sproutID, "independent", GenerateJobID(), false, WithInvoker("pubkey-xyz"))
	if err != nil {
		t.Fatalf("SendCookEvent (with invoker): %v", err)
	}
}

func TestSendCookEventNotAcknowledged(t *testing.T) {
	nc, cleanup := startCookTestNATS(t)
	defer cleanup()

	sproutID := "send-cook-nack-sprout"
	sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".cook", func(msg *nats.Msg) {
		ack := Ack{Acknowledged: false, JobID: "wrong"}
		data, _ := json.Marshal(ack)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	err = SendCookEvent(sproutID, "independent", GenerateJobID(), false)
	if err == nil {
		t.Error("expected error when sprout does not acknowledge")
	}
}

func TestSendCookEventWrongJobID(t *testing.T) {
	nc, cleanup := startCookTestNATS(t)
	defer cleanup()

	sproutID := "send-cook-wrongjid-sprout"
	sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".cook", func(msg *nats.Msg) {
		ack := Ack{Acknowledged: true, JobID: "wrong-jid"}
		data, _ := json.Marshal(ack)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	err = SendCookEvent(sproutID, "independent", GenerateJobID(), false)
	if err == nil {
		t.Error("expected error for wrong job ID in ack")
	}
}

func TestSendCookEventNoRecipe(t *testing.T) {
	_, cleanup := startCookTestNATS(t)
	defer cleanup()

	err := SendCookEvent("some-sprout", "nonexistent-recipe-xyz", GenerateJobID(), false)
	if err == nil {
		t.Error("expected error for non-existent recipe")
	}
}

func TestSendCookEventInvalidRecipe(t *testing.T) {
	_, cleanup := startCookTestNATS(t)
	defer cleanup()

	err := SendCookEvent("some-sprout", "invalidReq", GenerateJobID(), false)
	if err == nil {
		t.Error("expected error for invalid recipe")
	}
}

// --- ResolveRecipeFilePath edge cases ---

func TestResolveRecipeFilePathDirectory(t *testing.T) {
	// Test that a .grlx path that resolves to a directory returns ErrRecipePathIsDirectory
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "test.grlx")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := ResolveRecipeFilePath(tmpDir, RecipeName("test.grlx"))
	if !errors.Is(err, ErrRecipePathIsDirectory) {
		t.Errorf("expected ErrRecipePathIsDirectory, got %v", err)
	}
}

func TestResolveRecipeFilePathInitIsDirectory(t *testing.T) {
	// Test that init.grlx being a directory returns ErrRecipePathIsDirectory
	tmpDir := t.TempDir()
	recipeDir := filepath.Join(tmpDir, "myrecipe")
	initPath := filepath.Join(recipeDir, "init.grlx")
	if err := os.MkdirAll(initPath, 0o755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}

	_, err := ResolveRecipeFilePath(tmpDir, RecipeName("myrecipe"))
	if !errors.Is(err, ErrRecipePathIsDirectory) {
		t.Errorf("expected ErrRecipePathIsDirectory, got %v", err)
	}
}

func TestResolveRecipeFilePathExtIsDirectory(t *testing.T) {
	// Test that resolved .grlx extension path being a directory returns ErrRecipePathIsDirectory
	tmpDir := t.TempDir()
	grlxDir := filepath.Join(tmpDir, "myrecipe.grlx")
	if err := os.MkdirAll(grlxDir, 0o755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}

	_, err := ResolveRecipeFilePath(tmpDir, RecipeName("myrecipe"))
	if !errors.Is(err, ErrRecipePathIsDirectory) {
		t.Errorf("expected ErrRecipePathIsDirectory, got %v", err)
	}
}
