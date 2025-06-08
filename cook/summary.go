package cook

import "github.com/gogrlx/grlx/v2/types"

func SummarizeSteps(steps []types.SproutStepCompletion) map[string]types.Summary {
	summary := make(map[string]types.Summary)
	for _, step := range steps {
		if _, ok := summary[step.SproutID]; !ok {
			summary[step.SproutID] = types.Summary{}
		}
		stepSummary := summary[step.SproutID]
		if step.CompletedStep.ChangesMade {
			stepSummary.Changes += 1
		}
		switch step.CompletedStep.CompletionStatus {
		case types.StepCompleted:
			stepSummary.Succeeded += 1
		case types.StepFailed:
			stepSummary.Failures += 1
			stepSummary.Errors = append(stepSummary.Errors, step.CompletedStep.Error)
		}
		summary[step.SproutID] = stepSummary
	}

	return summary
}
