package natsapi

import (
	"context"
	"sort"
	"time"

	"github.com/gogrlx/grlx/v2/internal/log"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// StartCohortRefresher launches a background goroutine that periodically
// refreshes all dynamic cohort memberships. It recalculates membership
// by evaluating property matches against currently known sprouts.
//
// The refresher runs every interval. Pass a zero or negative interval to
// disable scheduled refresh. Cancel the context to stop the goroutine.
// The returned channel is closed when the goroutine exits.
func StartCohortRefresher(ctx context.Context, interval time.Duration) <-chan struct{} {
	done := make(chan struct{})
	if interval <= 0 {
		log.Info("Cohort scheduled refresh disabled (interval <= 0)")
		close(done)
		return done
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Infof("Cohort scheduled refresh started (every %s)", interval)

		for {
			select {
			case <-ctx.Done():
				log.Info("Cohort scheduled refresh stopped")
				return
			case <-ticker.C:
				refreshAllCohorts()
			}
		}
	}()

	return done
}

// refreshAllCohorts gathers current sprout IDs from the PKI store and
// refreshes every registered cohort's cached membership.
func refreshAllCohorts() {
	if cohortRegistry == nil {
		return
	}

	names := cohortRegistry.List()
	if len(names) == 0 {
		return
	}

	allKeys := pki.ListNKeysByType()
	allSproutIDs := make([]string, 0, len(allKeys.Accepted.Sprouts))
	for _, km := range allKeys.Accepted.Sprouts {
		allSproutIDs = append(allSproutIDs, km.SproutID)
	}
	sort.Strings(allSproutIDs)

	results, err := cohortRegistry.RefreshAll(allSproutIDs)
	if err != nil {
		log.Errorf("Scheduled cohort refresh error: %v", err)
		return
	}

	log.Debugf("Scheduled cohort refresh completed: %d cohort(s) refreshed", len(results))
}
