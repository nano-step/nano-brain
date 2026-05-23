package harvest

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// Runner periodically invokes an OpenCodeHarvester.
type Runner struct {
	harvester *OpenCodeHarvester
	enqueuer  ChunkEnqueuer
	interval  time.Duration
	logger    zerolog.Logger
}

// NewRunner creates a Runner that calls HarvestAll at the given interval.
func NewRunner(harvester *OpenCodeHarvester, enqueuer ChunkEnqueuer, interval time.Duration, logger zerolog.Logger) *Runner {
	return &Runner{
		harvester: harvester,
		enqueuer:  enqueuer,
		interval:  interval,
		logger:    logger.With().Str("component", "harvest-runner").Logger(),
	}
}

// Run executes an immediate harvest then ticks at the configured interval.
// It returns nil on context cancellation.
func (r *Runner) Run(ctx context.Context) error {
	r.tick(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("harvest runner stopping")
			return nil
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *Runner) tick(ctx context.Context) {
	harvested, skipped, errCount := r.harvester.HarvestAll(ctx, r.enqueuer)
	r.logger.Info().
		Int("harvested", harvested).
		Int("skipped", skipped).
		Int("errors", errCount).
		Msg("harvest cycle complete")
}
