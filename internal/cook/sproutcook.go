package cook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gogrlx/grlx/v2/internal/log"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

var (
	ErrCookTimeout     = errors.New("recipe cooking timed out")
	ErrStalled         = errors.New("no steps are in progress")
	ErrRequisiteNotMet = errors.New("requisite not met")
	nonCCM             = sync.Mutex{}
)

// DefaultCookTimeout is the maximum time allowed for a recipe envelope to
// complete all steps. If all steps are not finished within this duration,
// the operation is cancelled and an error is returned.
const DefaultCookTimeout = 30 * time.Minute

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
			b, marshalErr := json.Marshal(completion)
			if marshalErr != nil {
				log.Errorf("failed to marshal step completion: %v", marshalErr)
			}

			conn.Publish("grlx.cook."+pki.GetSproutID()+"."+envelope.JobID, b)
			log.Infof("Step %s completed with status %v", completion.ID, completion)
			wg.Done()
			logStepResult(envelope.JobID, completion)
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
					entry := completionMap[id]
					entry.CompletionStatus = StepFailed
					completionMap[id] = entry
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
				entry := completionMap[id]
				entry.CompletionStatus = StepInProgress
				completionMap[id] = entry
				// all requisites are met, so start the step in a goroutine
				go func(ctx context.Context, step Step, cChan chan StepCompletion, testMode bool) {
					started := time.Now()
					// use the ingredient package to load and cook the step
					ingredient, err := NewRecipeCooker(step.ID, step.Ingredient, step.Method, step.Properties)
					if err != nil {
						cChan <- StepCompletion{
							ID:               step.ID,
							CompletionStatus: StepFailed,
							Started:          started,
							Duration:         time.Since(started),
							Error:            err,
						}
						return
					}
					var res Result
					if testMode {
						res, err = ingredient.Test(ctx)
					} else {
						res, err = ingredient.Apply(ctx)
					}

					duration := time.Since(started)
					// in test mode, res.Changed and res.Notes may not be populated
					var changed bool
					var notes []string
					if !testMode {
						changed = res.Changed
						for _, note := range res.Notes {
							notes = append(notes, note.String())
						}
					}
					status := StepCompleted
					if !res.Succeeded {
						status = StepFailed
					}
					cChan <- StepCompletion{
						ID:               step.ID,
						CompletionStatus: status,
						ChangesMade:      changed,
						Changes:          notes,
						Started:          started,
						Duration:         duration,
						Error:            err,
					}
				}(ctx, stepMap[id], completionChan, envelope.Test)
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
			b, marshalErr := json.Marshal(completion)
			if marshalErr != nil {
				log.Errorf("failed to marshal step completion: %v", marshalErr)
			}

			conn.Publish("grlx.cook."+pki.GetSproutID()+"."+envelope.JobID, b)
			log.Info("All steps completed")
			return nil
		case <-time.After(DefaultCookTimeout):
			cancel()
			log.Errorf("recipe %s timed out after %v", envelope.JobID, DefaultCookTimeout)
			completion := StepCompletion{
				ID:               StepID(fmt.Sprintf("timeout-%s", envelope.JobID)),
				CompletionStatus: StepFailed,
				Error:            ErrCookTimeout,
			}
			b, marshalErr := json.Marshal(completion)
			if marshalErr != nil {
				log.Errorf("failed to marshal timeout completion: %v", marshalErr)
			}
			conn.Publish("grlx.cook."+pki.GetSproutID()+"."+envelope.JobID, b)
			return ErrCookTimeout
		}
	}
}

// logStepResult writes a step completion result to the local job log directory,
// organized by JID. Each step is appended as a JSON line to <JobLogDir>/<JID>.jsonl.
func logStepResult(jobID string, completion StepCompletion) {
	if config.JobLogDir == "" {
		return
	}
	if err := os.MkdirAll(config.JobLogDir, 0o700); err != nil {
		log.Errorf("failed to create job log directory: %v", err)
		return
	}
	logFile := filepath.Join(config.JobLogDir, fmt.Sprintf("%s.jsonl", jobID))
	b, err := json.Marshal(completion)
	if err != nil {
		log.Errorf("failed to marshal step completion for logging: %v", err)
		return
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Errorf("failed to open job log file %s: %v", logFile, err)
		return
	}
	defer f.Close()
	f.Write(b)
	f.WriteString("\n")
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
