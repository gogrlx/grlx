package cook

// This code will subscribe to the jobs topic and record the jobs
// to flat files in the jobs directory.  The files will be named
// with the job id and will contain the job data in jsonL format.

// The jobs directory will be created if it does not exist.

// Jobs are stored in triplicate: in the jobs directory on the farmer,
// in the jobs directory on the sprout, and in the jobs directory on
// the cli user's machine.
// Jobs can be retrieved from the farmer with the grlx job command.

// TODO configure expiration time for job data on the sprout and farmer

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/types"
)

// Job represents a job

type Job struct {
	ID      string         `json:"id"`
	Results []types.Result `json:"results"`
	Sprout  string         `json:"sprout"`
	Summary types.Summary  `json:"summary"`
}

var natsConn *nats.Conn

func logJobs() {
	// Create the jobs directory if it does not exist
	if _, err := os.Stat("jobs"); os.IsNotExist(err) {
		os.Mkdir("jobs", 0o755)
	}

	// Subscribe to the jobs topic
	sub, err := natsConn.SubscribeSync("jobs")
	if err != nil {
		log.Fatal(err)
	}

	// Read messages from the jobs topic
	for {
		msg, err := sub.NextMsg(10 * time.Second)
		if err != nil {
			log.Fatal(err)
		}

		// Get the job data
		var job Job
		err = json.Unmarshal(msg.Data, &job)
		if err != nil {
			log.Fatal(err)
		}

		// Create the job file
		jobFile := filepath.Join("jobs", fmt.Sprintf("%s.jsonl", job.ID))
		f, err := os.Create(jobFile)
		if err != nil {
			log.Fatal(err)
		}

		// Write the job data to the file
		b, err := json.Marshal(job)
		if err != nil {
			log.Fatal(err)
		}
		_, err = f.Write(b)
		if err != nil {
			log.Fatal(err)
		}
		_, err = f.WriteString("\n")
		if err != nil {
			log.Fatal(err)
		}

		// Close the file
		err = f.Close()
		if err != nil {
			log.Fatal(err)
		}

		// Log the job
		log.Printf("Job %s received\n", job.ID)

		// Acknowledge the message
		err = msg.Ack()
		if err != nil {
			log.Fatal(err)
		}
	}
}
