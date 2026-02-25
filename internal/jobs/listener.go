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

	"github.com/nats-io/nats.go"
	"github.com/taigrr/log-socket/log"

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
