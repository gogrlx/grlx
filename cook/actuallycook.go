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
	stepMap := map[types.StepID]types.Step{}
	for _, step := range envelope.Steps {
		stepMap[step.ID] = step
	}
	for _, step := range envelope.Steps {
		completionMap[step.ID] = StepCompletion{
			ID:               step.ID,
			CompletionStatus: NotStarted,
			ChangesMade:      false,
		}
	}
	wg := sync.WaitGroup{}
	wg.Add(len(envelope.Steps))
	completionChan := make(chan StepCompletion, 5)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		wg.Wait()
		cancel()
	}()
	for {
		select {
		case completion := <-completionChan:
			completionMap[completion.ID] = completion
			for id, step := range completionMap {
				if step.CompletionStatus != NotStarted {
					continue
				}
				requisitesMet, err := RequisitesAreMet(stepMap[id])
				if err != nil {
					// TODO here broadcast the error on the bus and mark the step as failed
					wg.Done()
				}
				if !requisitesMet {
					continue
				}
				go func() {
					// TODO here use the ingredient package to load and cook the step
					wg.Done()
				}()
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func RequisitesAreMet(step types.Step) (bool, error) {
	return false, nil
}
