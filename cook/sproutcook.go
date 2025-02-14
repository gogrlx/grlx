package cook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/ingredients"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/types"
)

var (
	ErrStalled         = errors.New("no steps are in progress")
	ErrRequisiteNotMet = errors.New("requisite not met")
	nonCCM             = sync.Mutex{}
)

func CookRecipeEnvelope(envelope types.RecipeEnvelope) error {
	nonCCM.Lock()
	defer nonCCM.Unlock()
	log.Tracef("received new envelope: %v", envelope)

	completionMap := map[types.StepID]types.StepCompletion{}
	for _, step := range envelope.Steps {
		completionMap[step.ID] = types.StepCompletion{
			ID:               step.ID,
			CompletionStatus: types.StepNotStarted,
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
	completionChan := make(chan types.StepCompletion, 1)
	completionChan <- types.StepCompletion{
		ID:               types.StepID(fmt.Sprintf("start-%s", envelope.JobID)),
		CompletionStatus: types.StepCompleted,
		ChangesMade:      false,
		Changes:          nil,
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
			b, _ := json.Marshal(completion)

			conn.Publish("grlx.cook."+pki.GetSproutID()+"."+envelope.JobID, b)
			log.Infof("Step %s completed with status %v", completion.ID, completion)
			wg.Done()
			// TODO also collect the results of the step and store them into a log folder by JID
			completionMap[completion.ID] = completion
			noneInProgress := true
			for id, step := range completionMap {
				if step.CompletionStatus == types.StepInProgress {
					noneInProgress = false
				}
				if step.CompletionStatus != types.StepNotStarted {
					continue
				}
				// mark the step as in progress
				requisitesMet, err := RequisitesAreMet(stepMap[id], completionMap)
				if err != nil {
					t := completionMap[id]
					t.CompletionStatus = types.StepFailed
					completionMap[id] = t
					go func(cChan chan types.StepCompletion, id types.StepID, err error) {
						cChan <- types.StepCompletion{
							ID:               id,
							CompletionStatus: types.StepFailed,
							Error:            err,
						}
					}(completionChan, id, err)
					continue
				}
				if !requisitesMet {
					continue
				}
				t := completionMap[id]
				t.CompletionStatus = types.StepInProgress
				completionMap[id] = t
				// all requisites are met, so start the step in a goroutine
				go func(step types.Step, cChan chan types.StepCompletion) {
					// use the ingredient package to load and cook the step
					ingredient, err := ingredients.NewRecipeCooker(step.ID, step.Ingredient, step.Method, step.Properties)
					if err != nil {
						cChan <- types.StepCompletion{
							ID:               step.ID,
							CompletionStatus: types.StepFailed,
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

					notes := []string{}
					for _, change := range res.Notes {
						notes = append(notes, change.String())
					}
					if res.Succeeded {
						cChan <- types.StepCompletion{
							ID:               step.ID,
							CompletionStatus: types.StepCompleted,
							ChangesMade:      res.Changed,
							Changes:          notes,
							Error:            err,
						}
					} else {
						cChan <- types.StepCompletion{
							ID:               step.ID,
							CompletionStatus: types.StepFailed,
							ChangesMade:      res.Changed,
							Changes:          notes,
							Error:            err,
						}
					}
				}(stepMap[id], completionChan)
				noneInProgress = false
			}
			if noneInProgress {
				// no steps are in progress, so we're done
				log.Debug("No steps are in progress")
			}
		// All steps are done, so context will be cancelled and we'll exit
		case <-ctx.Done():
			completion := types.StepCompletion{
				ID:               types.StepID(fmt.Sprintf("completed-%s", envelope.JobID)),
				CompletionStatus: types.StepCompleted,
				ChangesMade:      false,
				Changes:          nil,
			}
			b, _ := json.Marshal(completion)

			conn.Publish("grlx.cook."+pki.GetSproutID()+"."+envelope.JobID, b)
			log.Info("All steps completed")
			return nil
			// TODO add a timeout case
		}
	}
}

// RequisitesAreMet returns true if all of the requisites for the given step are met
// All top-level requisites are ANDed together, and meta states can be combined with an ANY clauses
// to use OR logic instead
func RequisitesAreMet(step types.Step, completionMap map[types.StepID]types.StepCompletion) (bool, error) {
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
				if reqStatus.CompletionStatus == types.StepCompleted || reqStatus.CompletionStatus == types.StepFailed {
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
				if reqStatus.CompletionStatus == types.StepCompleted {
					return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, string(req)))
				} else if reqStatus.CompletionStatus != types.StepFailed {
					// if the step is not completed or failed, then the requisite is not met (yet)
					unmet = true
				}
			}
		case types.Require:
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == types.StepFailed {
					return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, string(req)))
				} else if reqStatus.CompletionStatus != types.StepCompleted {
					unmet = true
				}
			}

		case types.OnChangesAny:
			met := false
			pendingRemaining := false
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == types.StepCompleted || reqStatus.CompletionStatus == types.StepFailed {
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
				if reqStatus.CompletionStatus == types.StepFailed {
					met = true
				} else if reqStatus.CompletionStatus != types.StepCompleted {
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
				if reqStatus.CompletionStatus == types.StepCompleted {
					met = true
				} else if reqStatus.CompletionStatus != types.StepFailed {
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
