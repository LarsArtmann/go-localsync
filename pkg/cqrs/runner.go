package cqrs

import (
	"context"
	"fmt"
	"time"

	"github.com/larsartmann/go-cqrs-lite/core/event"
	cqrsprojection "github.com/larsartmann/go-cqrs-lite/projection"
)

func startOutboxPublisher(
	outbox event.Outbox,
	bus event.Bus,
) (*event.OutboxPublisher, error) {
	if outbox == nil {
		return nil, nil //nolint:nilnil // intentional: nil publisher signals no outbox needed
	}

	publisher, err := event.NewOutboxPublisher(outbox, bus,
		event.WithPollInterval(time.Second),
		event.WithBatchSize(100),
	)
	if err != nil {
		return nil, fmt.Errorf("create outbox publisher: %w", err)
	}

	if startErr := publisher.Start(); startErr != nil {
		return nil, fmt.Errorf("start outbox publisher: %w", startErr)
	}

	return publisher, nil
}

func startProjectionRunner(
	sr storeResult,
	checkpointStore event.CheckpointStore,
	proj event.Projection,
) (context.CancelFunc, error) {
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
		_ = runner.Run(ctx)
	}()

	return cancel, nil
}

func startInMemoryRunner(
	bus event.Bus,
	checkpointStore event.CheckpointStore,
	proj event.Projection,
) error {
	runner, err := event.NewInMemoryRunner(checkpointStore)
	if err != nil {
		return fmt.Errorf("create in-memory projection runner: %w", err)
	}

	if regErr := runner.Register(proj); regErr != nil {
		return fmt.Errorf("register projector: %w", regErr)
	}

	if subErr := bus.SubscribeAll(runner.Handle); subErr != nil {
		return fmt.Errorf("subscribe projection runner: %w", subErr)
	}

	return nil
}
