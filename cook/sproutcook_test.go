package cook

import (
	"errors"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestRequisitesAreMet(t *testing.T) {
	// TODO

	completionmap := map[types.StepID]types.StepCompletion{
		"failed": {
			ID:               "failed",
			CompletionStatus: types.StepFailed,
		},
		"succeeded": {
			ID:               "succeeded",
			CompletionStatus: types.StepCompleted,
		},
		"inprogress": {
			ID:               "inprogress",
			CompletionStatus: types.StepInProgress,
		},
		"notstarted": {
			ID:               "notstarted",
			CompletionStatus: types.StepNotStarted,
		},
	}

	testCases := []struct {
		id         string
		requisites types.RequisiteSet
		expected   bool
		err        error
	}{
		{
			id:         "no reqs",
			requisites: types.RequisiteSet{},
			expected:   true, err: nil,
		},
		{
			id: "one requisite, not met",
			requisites: types.RequisiteSet{types.Requisite{
				Condition: types.Require,
				StepIDs:   []types.StepID{"failed"},
			}},
			expected: false, err: ErrRequisiteNotMet,
		},
		{
			id: "one requisite, met",
			requisites: types.RequisiteSet{types.Requisite{
				Condition: types.Require,
				StepIDs:   []types.StepID{"succeeded"},
			}},
			expected: true, err: nil,
		},
		{
			id: "one requisite, in progress",
			requisites: types.RequisiteSet{types.Requisite{
				Condition: types.Require,
				StepIDs:   []types.StepID{"inprogress"},
			}},
			expected: false,
			err:      nil,
		},
		{
			id: "two requisites, one not met",
			requisites: types.RequisiteSet{
				types.Requisite{
					Condition: types.Require,
					StepIDs:   []types.StepID{"succeeded", "failed"},
				},
			},
			expected: false,
			err:      ErrRequisiteNotMet,
		},
		{
			id: "two requisites, one met, one pending",
			requisites: types.RequisiteSet{
				types.Requisite{
					Condition: types.Require,
					StepIDs:   []types.StepID{"succeeded", "inprogress"},
				},
			},
			expected: false, err: nil,
		},
		{
			id: "two anyrequisites, one met, one pending",
			requisites: types.RequisiteSet{
				types.Requisite{
					Condition: types.RequireAny,
					StepIDs:   []types.StepID{"succeeded", "inprogress"},
				},
			},
			expected: true, err: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			met, err := RequisitesAreMet(types.Step{Requisites: tc.requisites}, completionmap)
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v, got %v", tc.err, err)
			}
			if met != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, met)
			}
		})
	}
}
