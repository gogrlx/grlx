package cook

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/types"
)

type CompletionStatus int

var (
	ErrStalled         = errors.New("no steps are in progress")
	ErrRequisiteNotMet = errors.New("requisite not met")
)

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
	log.Debugf("received new envelope: %v", envelope)
	completionMap := map[types.StepID]StepCompletion{}
	for _, step := range envelope.Steps {
		completionMap[step.ID] = StepCompletion{
			ID:               step.ID,
			CompletionStatus: NotStarted,
			ChangesMade:      false,
			Changes:          nil,
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
			ec.Publish("grlx.cook."+pki.GetSproutID()+"."+envelope.JobID, completion)
			log.Infof("Step %s completed with status %v", completion.ID, completion)
			wg.Done()
			// TODO also collect the results of the step and store them into a log folder by JID
			completionMap[completion.ID] = completion
			noneInProgress := true
			for id, step := range completionMap {
				if step.CompletionStatus == InProgress {
					noneInProgress = false
				}
				if step.CompletionStatus != NotStarted {
					continue
				}
				// mark the step as in progress
				requisitesMet, err := RequisitesAreMet(stepMap[id], completionMap)
				if err != nil {
					t := completionMap[id]
					t.CompletionStatus = Failed
					completionMap[id] = t
					go func(cChan chan StepCompletion, id types.StepID, err error) {
						cChan <- StepCompletion{
							ID:               id,
							CompletionStatus: Failed,
							Error:            err,
						}
					}(completionChan, id, err)
					continue
				}
				if !requisitesMet {
					continue
				}
				t := completionMap[id]
				t.CompletionStatus = InProgress
				completionMap[id] = t
				// all requisites are met, so start the step in a goroutine
				go func(step types.Step, cChan chan StepCompletion) {
					// use the ingredient package to load and cook the step
					ingredient, err := ingredients.NewRecipeCooker(step.ID, step.Ingredient, step.Method, step.Properties)
					if err != nil {
						cChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Failed,
							Error:            err,
						}
						return
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
						cChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Completed,
							ChangesMade:      res.Changed,
							Changes:          res.Changes,
							Error:            err,
						}
					} else {
						cChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: Failed,
							ChangesMade:      res.Changed,
							Changes:          res.Changes,
							Error:            err,
						}
					}
				}(stepMap[id], completionChan)
				noneInProgress = false
			}
			if noneInProgress {
				// no steps are in progress, so we're done
				log.Print("No steps are in progress")
			}
		// All steps are done, so context will be cancelled and we'll exit
		case <-ctx.Done():
			log.Print("All steps completed")
			return nil
			// TODO add a timeout case
		}
	}
}

// RequisitesAreMet returns true if all of the requisites for the given step are met
// All top-level requisites are ANDed together, and meta states can be combined with an ANY clauses
// to use OR logic instead
func RequisitesAreMet(step types.Step, completionMap map[types.StepID]StepCompletion) (bool, error) {
	if len(step.Requisites) == 0 {
		return true, nil
	}
	unmet := false
	for _, reqSet := range step.Requisites {
		errStr := "%s requirement of %s not met"
		switch reqSet.Condition {
		case types.OnChanges:
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				// if the step is completed or failed, and no changes were made, then the requisite cannot be met
				if reqStatus.CompletionStatus == Completed || reqStatus.CompletionStatus == Failed {
					if !reqStatus.ChangesMade {
						return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, string(req)))
					}
				} else {
					// if the step is not completed or failed, then the requisite is not met (yet)
					unmet = true
				}
			}
		case types.OnFail:
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				// if the step is completed, then the requisite cannot be met
				if reqStatus.CompletionStatus == Completed {
					return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, string(req)))
				} else if reqStatus.CompletionStatus != Failed {
					// if the step is not completed or failed, then the requisite is not met (yet)
					unmet = true
				}
			}
		case types.Require:
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == Failed {
					return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, string(req)))
				} else if reqStatus.CompletionStatus != Completed {
					unmet = true
				}
			}

		case types.OnChangesAny:
			met := false
			pendingRemaining := false
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == Completed || reqStatus.CompletionStatus == Failed {
					if reqStatus.ChangesMade {
						met = true
					}
				} else {
					pendingRemaining = true
				}
			}
			if !pendingRemaining && !met {
				return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, "any"))
			}
			if !met {
				unmet = true
			}
		case types.OnFailAny:
			met := false
			pendingRemaining := false
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == Failed {
					met = true
				} else if reqStatus.CompletionStatus != Completed {
					pendingRemaining = true
				}
			}
			if !pendingRemaining && !met {
				return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, "any"))
			}
			if !met {
				unmet = true
			}
		case types.RequireAny:
			met := false
			pendingRemaining := false
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == Completed {
					met = true
				} else if reqStatus.CompletionStatus != Failed {
					pendingRemaining = true
				}
			}
			if !pendingRemaining && !met {
				return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, "any"))
			}
			if !met {
				unmet = true
			}
		default:
			return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf("unknown requisite condition %s", reqSet.Condition))
		}
	}
	return !unmet, nil
}
