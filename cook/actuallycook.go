package cook

import (
	"context"
	"sync"

	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/pki"
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
	Changes          any
	Error            error
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
	wg.Add(len(envelope.Steps) + 1)
	completionChan := make(chan StepCompletion, 1)
	completionChan <- StepCompletion{
		ID:               types.StepID("start"),
		CompletionStatus: Completed,
	}
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
			ec.Publish("grlx.cook."+envelope.JobID+"."+pki.GetSproutID(), completion)
			log.Noticef("Step %s completed with status %v", completion.ID, completion)
			// TODO also collect the results of the step and store them into a log folder by JID
			completionMap[completion.ID] = completion
			for id, step := range completionMap {
				if step.CompletionStatus != NotStarted {
					continue
				}
				requisitesMet, err := RequisitesAreMet(stepMap[id], completionMap)
				if err != nil {
					completionChan <- StepCompletion{
						ID:               id,
						CompletionStatus: Failed,
						Error:            err,
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
					// use the ingredient package to load and cook the step
					ingredient, err := ingredients.NewRecipeCooker(step.ID, step.Ingredient, step.Method, step.Properties)
					if err != nil {
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Failed,
						}
					}
					var res types.Result
					// TODO allow for cancellation
					bgCtx := context.Background()
					// TODO make sure envelope.Test is set in grlx and farmer
					if envelope.Test {
						res, err = ingredient.Test(bgCtx)
					} else {
						res, err = ingredient.Apply(bgCtx)
					}
					if res.Succeeded {
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Completed,
							ChangesMade:      res.Changed,
							Changes:          res.Changes,
						}
					} else if res.Failed {
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Failed,
							ChangesMade:      res.Changed,
							Changes:          res.Changes,
						}
					} else if err != nil {
						completionChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Completed,
							Error:            err,
							// res might be nil if err is not nil
							// so don't try to access res.Changed or res.Changes
						}
					}
				}(stepMap[id])
			}
		// All steps are done, so context will be cancelled and we'll exit
		case <-ctx.Done():
			return nil
			// TODO add a timeout case
		}
	}
}

// TODO this is a stub
// RequisitesAreMet returns true if all of the requisites for the given step are met
func RequisitesAreMet(step types.Step, completionMap map[types.StepID]StepCompletion) (bool, error) {
	return false, nil
}
