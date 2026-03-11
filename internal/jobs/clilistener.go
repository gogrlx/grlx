package jobs

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/log"
	"github.com/nats-io/nats.go"
)

// CLIListener subscribes to NATS job result subjects and records them
// into a CLIStore. It is used by the grlx CLI to maintain a local
// copy of job execution data.
type CLIListener struct {
	store   *CLIStore
	nc      *nats.Conn
	sub     *nats.Subscription
	userKey string
}

// NewCLIListener creates a listener that will record job results into the
// given CLIStore. userKey is the public key of the current CLI user.
func NewCLIListener(store *CLIStore, nc *nats.Conn, userKey string) *CLIListener {
	return &CLIListener{
		store:   store,
		nc:      nc,
		userKey: userKey,
	}
}

// SubscribeAll subscribes to all job completion events. This is useful for
// background recording of all job activity visible to the current user.
func (l *CLIListener) SubscribeAll() error {
	sub, err := l.nc.Subscribe("grlx.cook.*.*", l.handleStepCompletion)
	if err != nil {
		return err
	}
	l.sub = sub
	return nil
}

// SubscribeJob subscribes only to completion events for a specific JID.
func (l *CLIListener) SubscribeJob(jid string) error {
	topic := "grlx.cook.*." + jid
	sub, err := l.nc.Subscribe(topic, l.handleStepCompletion)
	if err != nil {
		return err
	}
	l.sub = sub
	return nil
}

// RecordJobInit records the initial metadata for a job that the current
// user just initiated (via cook command).
func (l *CLIListener) RecordJobInit(jid, recipe string, sproutIDs []string) {
	for _, sproutID := range sproutIDs {
		meta := CLIJobMeta{
			JID:       jid,
			SproutID:  sproutID,
			Recipe:    recipe,
			UserKey:   l.userKey,
			CreatedAt: time.Now(),
		}
		if err := l.store.RecordJobStart(meta); err != nil {
			log.Errorf("CLI job store: failed to record job start for %s/%s: %v", sproutID, jid, err)
		}
	}
}

// Stop unsubscribes the listener from NATS.
func (l *CLIListener) Stop() {
	if l.sub != nil {
		l.sub.Unsubscribe()
	}
}

func (l *CLIListener) handleStepCompletion(msg *nats.Msg) {
	// Subject: grlx.cook.<sproutID>.<jid>
	parts := strings.Split(msg.Subject, ".")
	if len(parts) < 4 {
		return
	}
	sproutID := parts[2]
	jid := parts[3]

	var step cook.StepCompletion
	if err := json.Unmarshal(msg.Data, &step); err != nil {
		log.Errorf("CLI job store: failed to unmarshal step: %v", err)
		return
	}

	if err := l.store.AppendStep(sproutID, jid, step); err != nil {
		log.Errorf("CLI job store: failed to append step for %s/%s: %v", sproutID, jid, err)
	}
}
