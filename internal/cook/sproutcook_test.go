package cook

import (
	"errors"
	"testing"
)

func TestRequisitesAreMet(t *testing.T) {
	// TODO

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
