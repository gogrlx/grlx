package cook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/v2/internal/pki"
)

var (
	ErrStalled         = errors.New("no steps are in progress")
	ErrRequisiteNotMet = errors.New("requisite not met")
	nonCCM             = sync.Mutex{}
)

func CookRecipeEnvelope(envelope RecipeEnvelope) error {
	nonCCM.Lock()
	defer nonCCM.Unlock()
	log.Tracef("received new envelope: %v", envelope)

	completionMap := map[StepID]StepCompletion{}
	for _, step := range envelope.Steps {
		completionMap[step.ID] = StepCompletion{
			ID:               step.ID,
			CompletionStatus: StepNotStarted,
			ChangesMade:      false,
			Changes:          nil,
		}
	}
	stepMap := map[StepID]Step{}
	for _, step := range envelope.Steps {
		stepMap[step.ID] = step
	}
	// create a wait group and a channel to receive step completions
	wg := sync.WaitGroup{}
	wg.Add(len(envelope.Steps) + 1)
	completionChan := make(chan StepCompletion, 1)
	completionChan <- StepCompletion{
		ID:               StepID(fmt.Sprintf("start-%s", envelope.JobID)),
		CompletionStatus: StepCompleted,
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
				if step.CompletionStatus == StepInProgress {
					noneInProgress = false
				}
				if step.CompletionStatus != StepNotStarted {
					continue
				}
				// mark the step as in progress
				requisitesMet, err := RequisitesAreMet(stepMap[id], completionMap)
				if err != nil {
					t := completionMap[id]
					t.CompletionStatus = StepFailed
					completionMap[id] = t
					go func(cChan chan StepCompletion, id StepID, err error) {
						cChan <- StepCompletion{
							ID:               id,
							CompletionStatus: StepFailed,
							Error:            err,
						}
					}(completionChan, id, err)
					continue
				}
				if !requisitesMet {
					continue
				}
				t := completionMap[id]
				t.CompletionStatus = StepInProgress
				completionMap[id] = t
				// all requisites are met, so start the step in a goroutine
				go func(step Step, cChan chan StepCompletion) {
					// use the ingredient package to load and cook the step
					ingredient, err := NewRecipeCooker(step.ID, step.Ingredient, step.Method, step.Properties)
					if err != nil {
						cChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: StepFailed,
							Error:            err,
						}
						return
					}
					var res Result
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
						cChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: StepCompleted,
							ChangesMade:      res.Changed,
							Changes:          notes,
							Error:            err,
						}
					} else {
						cChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: StepFailed,
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
			completion := StepCompletion{
				ID:               StepID(fmt.Sprintf("completed-%s", envelope.JobID)),
				CompletionStatus: StepCompleted,
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
func RequisitesAreMet(step Step, completionMap map[StepID]StepCompletion) (bool, error) {
	if len(step.Requisites) == 0 {
		return true, nil
	}
	unmet := false
	for _, reqSet := range step.Requisites {
		errStr := "%s requirement of %s not met"
		switch reqSet.Condition {
		case OnChanges:
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				// if the step is completed or failed, and no changes were made, then the requisite cannot be met
				if reqStatus.CompletionStatus == StepCompleted || reqStatus.CompletionStatus == StepFailed {
					if !reqStatus.ChangesMade {
						return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, string(req)))
					}
				} else {
					// if the step is not completed or failed, then the requisite is not met (yet)
					unmet = true
				}
			}
		case OnFail:
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				// if the step is completed, then the requisite cannot be met
				if reqStatus.CompletionStatus == StepCompleted {
					return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, string(req)))
				} else if reqStatus.CompletionStatus != StepFailed {
					// if the step is not completed or failed, then the requisite is not met (yet)
					unmet = true
				}
			}
		case Require:
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == StepFailed {
					return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, string(req)))
				} else if reqStatus.CompletionStatus != StepCompleted {
					unmet = true
				}
			}

		case OnChangesAny:
			met := false
			pendingRemaining := false
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == StepCompleted || reqStatus.CompletionStatus == StepFailed {
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
		case OnFailAny:
			met := false
			pendingRemaining := false
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == StepFailed {
					met = true
				} else if reqStatus.CompletionStatus != StepCompleted {
					pendingRemaining = true
				}
			}
			if !pendingRemaining && !met {
				return false, errors.Join(ErrRequisiteNotMet, fmt.Errorf(errStr, reqSet.Condition, "any"))
			}
			if !met {
				unmet = true
			}
		case RequireAny:
			met := false
			pendingRemaining := false
			for _, req := range reqSet.StepIDs {
				reqStatus := completionMap[req]
				if reqStatus.CompletionStatus == StepCompleted {
					met = true
				} else if reqStatus.CompletionStatus != StepFailed {
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
