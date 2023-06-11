package cook

import "github.com/gogrlx/grlx/types"

type CompletionStatus int

const (
	NotStarted CompletionStatus = iota
	InProgress
	Completed
	Failed
)

type StepCompletion struct {
	ID               types.StepID
	CompletionStatus CompletionStatus
	ChangesMade      bool
}

func CookRecipeEnvelope(envelope types.RecipeEnvelope) error {
	completionMap := map[types.StepID]StepCompletion{}
	for _, step := range envelope.Steps {
		completionMap[step.ID] = StepCompletion{
			ID:               step.ID,
			CompletionStatus: NotStarted,
			ChangesMade:      false,
		}
	}
	completionChan := make(chan StepCompletion, 5)
	for {
		select {
		case completion := <-completionChan:
			completionMap[completion.ID] = completion
			go func() {
			}()
		}
	}
	return nil
}

func RequisitesAreMet(step types.Step) (bool, error) {
	return false, nil
}
