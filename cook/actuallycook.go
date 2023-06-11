package cook

import (
	"context"
	"sync"

	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/types"
)

type CompletionStatus int

const (
	NotStarted CompletionStatus = iota
	InProgress
	Completed
	Failed
)

// TODO store Changes struct here to allow for unified error/status reporting
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
	stepMap := map[types.StepID]types.Step{}
	for _, step := range envelope.Steps {
		stepMap[step.ID] = step
	}
	// create a wait group and a channel to receive step completions
	wg := sync.WaitGroup{}
	wg.Add(len(envelope.Steps))
	completionChan := make(chan StepCompletion, 5)
	ctx, cancel := context.WithCancel(context.Background())
	// spawn a goroutine to wait for all steps to complete and then cancel the context
	go func() {
		wg.Wait()
		cancel()
	}()
	for {
		select {
		// each time a step completes, check if any other steps can be started
		case completion := <-completionChan:
			completionMap[completion.ID] = completion
			for id, step := range completionMap {
				if step.CompletionStatus != NotStarted {
					continue
				}
				requisitesMet, err := RequisitesAreMet(stepMap[id], completionMap)
				if err != nil {
					// TODO here broadcast the error on the bus
					completionChan <- StepCompletion{
						ID:               id,
						CompletionStatus: Failed,
					}
					wg.Done()
					continue
				}
				if !requisitesMet {
					continue
				}
				// mark the step as in progress
				t := completionMap[id]
				t.CompletionStatus = InProgress
				completionMap[id] = t
				// all requisites are met, so start the step in a goroutine
				go func(step types.Step) {
					defer wg.Done()
					// TODO here use the ingredient package to load and cook the step
					ingredient, err := ingredients.NewRecipeCooker(step.ID, step.Ingredient, step.Method, step.Properties)
					if err != nil {
						// TODO here broadcast the error on the bus
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Failed,
						}
					}
					// TODO allow for cancellation
					// TODO check for Test v Apply
					res, err := ingredient.Apply(context.Background())
					if err != nil {
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Failed,
						}
					}
					if res.Succeeded {
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Completed,
						}
					} else if res.Failed {
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Failed,
						}
					} else {
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Completed,
						}
					}
				}(stepMap[id])
			}
		// All steps are done, so context will be cancelled and we'll exit
		case <-ctx.Done():
			return nil
		}
	}
}

// RequisitesAreMet returns true if all of the requisites for the given step are met
// TODO this is a stub
func RequisitesAreMet(step types.Step, completionMap map[types.StepID]StepCompletion) (bool, error) {
	return false, nil
}
