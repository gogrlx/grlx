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

	"github.com/nats-io/nats.go"
	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/types"
)

// Job represents a job

var ec *nats.EncodedConn

func RegisterEC(conn *nats.EncodedConn) {
	ec = conn
	_, err := ec.Subscribe("grlx.cook.>", logJobs)
	if err != nil {
		log.Error(err)
	}
	_, err = ec.Subscribe("grlx.sprouts.*.cook", logJobCreation)
	if err != nil {
		log.Error(err)
	}
}

func init() {
	// Create the jobs directory if it does not exist
	if _, err := os.Stat(config.JobLogDir); os.IsNotExist(err) {
		os.MkdirAll(config.JobLogDir, 0o700)
	}
}

func logJobCreation(msg *nats.Msg) {
}

func logJobs(msg *nats.Msg) {
	// Create the jobs directory if it does not exist
	if _, err := os.Stat("jobs"); os.IsNotExist(err) {
		os.Mkdir("jobs", 0o755)
	}

	// Subscribe to the jobs topic

	// Get the job data
	var job types.Job
	err := json.Unmarshal(msg.Data, &job)
	if err != nil {
		log.Error(err)
		return
	}
	f := &os.File{}
	// Create the job file
	jobFile := filepath.Join("jobs", fmt.Sprintf("%s.jsonl", job.ID))
	st, err := os.Stat(jobFile)
	if errors.Is(err, os.ErrNotExist) {
		// File does not exist, create it
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
	b, err := json.Marshal(job)
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
	log.Tracef("Job %s received\n", job.ID)

	// Acknowledge the message
	err = msg.Ack()
	if err != nil {
		log.Error(err)
	}
}
