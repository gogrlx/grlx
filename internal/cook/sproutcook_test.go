package cook

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil
}

func TestWriteStepCompletionLogLine(t *testing.T) {
	var buf bytes.Buffer
	completion := StepCompletion{
		ID:               "step-one",
		CompletionStatus: StepCompleted,
		ChangesMade:      true,
		Changes:          []string{"created file"},
	}

	if err := writeStepCompletionLogLine(&buf, completion); err != nil {
		t.Fatalf("writeStepCompletionLogLine returned error: %v", err)
	}

	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected JSONL output to end with newline, got %q", got)
	}
	if !strings.Contains(got, `"ID":"step-one"`) {
		t.Fatalf("expected JSONL output to contain step ID, got %q", got)
	}
}

func TestWriteStepCompletionLogLineReturnsWriteError(t *testing.T) {
	err := writeStepCompletionLogLine(failingWriter{}, StepCompletion{
		ID:               "step-one",
		CompletionStatus: StepCompleted,
	})

	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestWriteStepCompletionLogLineReturnsShortWrite(t *testing.T) {
	err := writeStepCompletionLogLine(shortWriter{}, StepCompletion{
		ID:               "step-one",
		CompletionStatus: StepCompleted,
	})

	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("expected short write error, got %v", err)
	}
}

func TestRequisitesAreMet(t *testing.T) {

	completionmap := map[StepID]StepCompletion{
		"failed": {
			ID:               "failed",
			CompletionStatus: StepFailed,
		},
		"succeeded": {
			ID:               "succeeded",
			CompletionStatus: StepCompleted,
		},
		"inprogress": {
			ID:               "inprogress",
			CompletionStatus: StepInProgress,
		},
		"notstarted": {
			ID:               "notstarted",
			CompletionStatus: StepNotStarted,
		},
	}

	testCases := []struct {
		id         string
		requisites RequisiteSet
		expected   bool
		err        error
	}{
		{
			id:         "no reqs",
			requisites: RequisiteSet{},
			expected:   true, err: nil,
		},
		{
			id: "one requisite, not met",
			requisites: RequisiteSet{Requisite{
				Condition: Require,
				StepIDs:   []StepID{"failed"},
			}},
			expected: false, err: ErrRequisiteNotMet,
		},
		{
			id: "one requisite, met",
			requisites: RequisiteSet{Requisite{
				Condition: Require,
				StepIDs:   []StepID{"succeeded"},
			}},
			expected: true, err: nil,
		},
		{
			id: "one requisite, in progress",
			requisites: RequisiteSet{Requisite{
				Condition: Require,
				StepIDs:   []StepID{"inprogress"},
			}},
			expected: false,
			err:      nil,
		},
		{
			id: "two requisites, one not met",
			requisites: RequisiteSet{
				Requisite{
					Condition: Require,
					StepIDs:   []StepID{"succeeded", "failed"},
				},
			},
			expected: false,
			err:      ErrRequisiteNotMet,
		},
		{
			id: "two requisites, one met, one pending",
			requisites: RequisiteSet{
				Requisite{
					Condition: Require,
					StepIDs:   []StepID{"succeeded", "inprogress"},
				},
			},
			expected: false, err: nil,
		},
		{
			id: "two anyrequisites, one met, one pending",
			requisites: RequisiteSet{
				Requisite{
					Condition: RequireAny,
					StepIDs:   []StepID{"succeeded", "inprogress"},
				},
			},
			expected: true, err: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			met, err := RequisitesAreMet(Step{Requisites: tc.requisites}, completionmap)
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v, got %v", tc.err, err)
			}
			if met != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, met)
			}
		})
	}
}
