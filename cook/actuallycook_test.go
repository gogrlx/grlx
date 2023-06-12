package cook

import (
	"errors"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestRequisitesAreMet(t *testing.T) {
	// TODO
	testCases := []struct {
		id            string
		step          types.Step
		completionmap map[types.StepID]StepCompletion
		expected      bool
		err           error
	}{
		{
			id: "no reqs",
			step: types.Step{
				ID:         "step1",
				Requisites: types.RequisiteSet{},
			},
			completionmap: map[types.StepID]StepCompletion{},
			expected:      true,
			err:           nil,
		},
		{
			id: "one requisite, not met",
			step: types.Step{
				ID: "step1",
				Requisites: types.RequisiteSet{types.Requisite{
					Condition: types.Require,
					StepIDs:   []types.StepID{"a"},
				}},
			},
			completionmap: map[types.StepID]StepCompletion{"a": {
				ID:               "a",
				CompletionStatus: Failed,
			}},
			expected: false,
			err:      ErrRequisiteNotMet,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			// TODO
			met, err := RequisitesAreMet(tc.step, tc.completionmap)
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v, got %v", tc.err, err)
			}
			if met != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, met)
			}
		})
	}
}
