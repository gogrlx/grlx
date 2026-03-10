package jobs

// This code will subscribe to the jobs topic and record the jobs
// to flat files in the jobs directory.  The files will be named
// with the job id and will contain the job data in jsonL format.

// The jobs directory will be created if it does not exist.

// Jobs will eventuall be stored in triplicate: in the jobs directory on the farmer,
// in the jobs directory on the sprout, and in the jobs directory on
// the cli user's machine. For now, they are only stored farmer-side.
// Jobs can be retrieved from the farmer with the grlx job command.

// TODO configure expiration time for job data on the sprout and farmer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/gogrlx/grlx/v2/pkg/log"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
)

// Job represents a job

var nc *nats.Conn

func RegisterNatsConn(conn *nats.Conn) {
	nc = conn
	_, err := nc.Subscribe("grlx.cook.*.*", logJobs)
	if err != nil {
		log.Error(err)
	}
	_, err = nc.Subscribe("grlx.sprouts.*.cook", logJobCreation)
	if err != nil {
		log.Error(err)
	}

	// Create the jobs directory if it does not exist
	// this cannot run in init, as the config is not yet loaded
	if _, err := os.Stat(config.JobLogDir); os.IsNotExist(err) {
		log.Noticef("Creating jobs directory %s\n", config.JobLogDir)
		err = os.MkdirAll(config.JobLogDir, 0o700)
		if err != nil {
			log.Error(err)
		}

	} else if err != nil {
		log.Error(err)
	}
}

func logJobCreation(msg *nats.Msg) {
	// Subject: grlx.sprouts.<sproutID>.cook
	tComponents := strings.Split(msg.Subject, ".")
	if len(tComponents) < 4 {
		log.Errorf("unexpected subject format for job creation: %s", msg.Subject)
		return
	}
	sprout := tComponents[2]

	var envelope cook.RecipeEnvelope
	if err := json.Unmarshal(msg.Data, &envelope); err != nil {
		log.Errorf("failed to unmarshal recipe envelope: %v", err)
		return
	}
	if envelope.JobID == "" {
		return
	}

	// Ensure the sprout directory exists.
	sproutDir := filepath.Join(config.JobLogDir, sprout)
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		log.Errorf("failed to create sprout job dir: %v", err)
		return
	}

	// Write a creation marker so the job appears in listings immediately,
	// even before any step completions arrive.
	jobFile := filepath.Join(sproutDir, fmt.Sprintf("%s.jsonl", envelope.JobID))
	if _, err := os.Stat(jobFile); err == nil {
		// File already exists (shouldn't happen, but be safe).
		return
	}

	// Write a "not started" step for each step in the envelope so the job
	// shows up with the correct total count right away.
	f, err := os.Create(jobFile)
	if err != nil {
		log.Errorf("failed to create job file for %s: %v", envelope.JobID, err)
		return
	}
	defer f.Close()

	for _, step := range envelope.Steps {
		placeholder := cook.StepCompletion{
			ID:               step.ID,
			CompletionStatus: cook.StepNotStarted,
			Started:          time.Now(),
		}
		b, marshalErr := json.Marshal(placeholder)
		if marshalErr != nil {
			log.Errorf("failed to marshal placeholder step: %v", marshalErr)
			continue
		}
		f.Write(b)
		f.WriteString("\n")
	}
	log.Noticef("job %s created for sprout %s (%d steps)", envelope.JobID, sprout, len(envelope.Steps))
}

func logJobs(msg *nats.Msg) {
	// Subscribe to the jobs topic
	tComponents := strings.Split(msg.Subject, ".")
	// subscription topic guaranteed to be in the form grlx.cook.<sprout>.<jid>
	sprout := tComponents[2]
	JID := tComponents[3]

	// Get the completedStep data
	var completedStep cook.StepCompletion
	err := json.Unmarshal(msg.Data, &completedStep)
	if err != nil {
		log.Error(err)
		return
	}
	f := &os.File{}
	// Create the job file
	jobFile := filepath.Join(config.JobLogDir, sprout, fmt.Sprintf("%s.jsonl", JID))
	log.Tracef("Job file: %s\n", jobFile)
	st, err := os.Stat(jobFile)
	if errors.Is(err, os.ErrNotExist) {
		// File does not exist, create it
		err = os.MkdirAll(filepath.Dir(jobFile), 0o700)
		if err != nil {
			log.Error(err)
			return
		}
		f, err = os.Create(jobFile)
		if err != nil {
			log.Error(err)
			return
		}
	} else if err != nil {
		log.Error(err)
		return
	} else if st.IsDir() {
		log.Errorf("job file %s is a directory", jobFile)
		return
	} else {
		// File exists, open it for appending
		f, err = os.OpenFile(jobFile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Error(err)
			return
		}
	}

	// Write the job data to the file
	b, err := json.Marshal(completedStep)
	if err != nil {
		log.Error(err)
		f.Close()
		return
	}
	_, err = f.Write(b)
	if err != nil {
		log.Error(err)
		f.Close()
		return
	}
	_, err = f.WriteString("\n")
	if err != nil {
		log.Error(err)
		f.Close()
		return

	}

	// Close the file
	err = f.Close()
	if err != nil {
		log.Error(err)
		return
	}

	// Log the job
	log.Tracef("Job %s received\n", completedStep.ID)
}
