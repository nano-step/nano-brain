package harvest

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type runnerTestHarvester struct{}

func (runnerTestHarvester) HarvestAll(context.Context, ChunkEnqueuer) (int, int, int) {
	return 0, 0, 0
}

type runnerTestEnqueuer struct{}

func (*runnerTestEnqueuer) Enqueue(uuid.UUID) bool { return true }

func TestNewRunnerNormalizesTypedNilEnqueuer(t *testing.T) {
	var enqueuer *runnerTestEnqueuer
	r := NewRunner(runnerTestHarvester{}, enqueuer, time.Second, zerolog.Nop())
	if r.enqueuer != nil {
		t.Fatal("typed nil enqueuer must normalize to nil")
	}
}

func TestNewRunnerPreservesNonNilEnqueuer(t *testing.T) {
	enqueuer := &runnerTestEnqueuer{}
	r := NewRunner(runnerTestHarvester{}, enqueuer, time.Second, zerolog.Nop())
	if r.enqueuer == nil {
		t.Fatal("non-nil enqueuer must be preserved")
	}
}
