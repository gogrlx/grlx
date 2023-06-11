package cook

import (
	"context"
	"sync"

	"github.com/gogrlx/grlx/types"
)

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
				// all requisites are met, so start the step in a goroutine
				go func(step types.Step) {
					// TODO here use the ingredient package to load and cook the step
					wg.Done()
					completionChan <- StepCompletion{
						ID:               step.ID,
						CompletionStatus: Completed,
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
