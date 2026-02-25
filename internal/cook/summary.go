package cook

func SummarizeSteps(steps []SproutStepCompletion) map[string]Summary {
	summary := make(map[string]Summary)
	for _, step := range steps {
		if _, ok := summary[step.SproutID]; !ok {
			summary[step.SproutID] = Summary{}
		}
		stepSummary := summary[step.SproutID]
		if step.CompletedStep.ChangesMade {
			stepSummary.Changes += 1
		}
		switch step.CompletedStep.CompletionStatus {
		case StepCompleted:
			stepSummary.Succeeded += 1
		case StepFailed:
			stepSummary.Failures += 1
			stepSummary.Errors = append(stepSummary.Errors, step.CompletedStep.Error)
		}
		summary[step.SproutID] = stepSummary
	}

	return summary
}
