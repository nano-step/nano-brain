package telemetry

import (
	"context"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type TelemetryWriter interface {
	InsertSearchTelemetry(ctx context.Context, arg sqlc.InsertSearchTelemetryParams) error
}

type Recorder struct {
	writer TelemetryWriter
	logger zerolog.Logger
}

func NewRecorder(writer TelemetryWriter, logger zerolog.Logger) *Recorder {
	return &Recorder{writer: writer, logger: logger}
}

func (r *Recorder) Record(ctx context.Context, query string, resultCount int, latencyMs int64, collection, workspace string) {
	go func() {
		err := r.writer.InsertSearchTelemetry(context.Background(), sqlc.InsertSearchTelemetryParams{
			WorkspaceHash: workspace,
			QueryText:     query,
			ResultCount:   int32(resultCount),
			LatencyMs:     int32(latencyMs),
			Collection:    collection,
		})
		if err != nil {
			r.logger.Warn().Err(err).Str("workspace", workspace).Msg("failed to record search telemetry")
		}
	}()
}
