package cqrs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v2"
	cqrsprojection "github.com/larsartmann/go-cqrs-lite/projection/v2"
)

func startProjectionRunner(
	sr storeResult,
	checkpointStore event.CheckpointStore,
	proj event.Projection,
) (context.CancelFunc, error) {
	if subErr := sr.bus.SubscribeAll(proj.Handle); subErr != nil {
		return nil, fmt.Errorf("subscribe projection: %w", subErr)
	}

	if sr.loader == nil {
		return func() {}, nil
	}

	runner, err := cqrsprojection.NewRunner(
		sr.loader, sr.bus, checkpointStore,
		cqrsprojection.WithRetry(3, 100*time.Millisecond),
	)
	if err != nil {
		return nil, fmt.Errorf("create projection runner: %w", err)
	}

	if regErr := runner.Register(proj); regErr != nil {
		return nil, fmt.Errorf("register projector: %w", regErr)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := runner.Run(ctx); err != nil {
			slog.Error("projection runner failed", "error", err)
		}
	}()

	return cancel, nil
}
