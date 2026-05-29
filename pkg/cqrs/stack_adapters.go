package cqrs

import (
	"context"
	"errors"

	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

func classifyAction(err error, eventCount int, wasNew bool, conflictWinner string) synclib.SyncAction {
	if err != nil {
		return synclib.ActionError
	}

	if eventCount > 1 {
		if conflictWinner == conflictWinnerLocal {
			return synclib.ActionConflictLocal
		}

		return synclib.ActionConflictRemote
	}

	if eventCount == 1 && wasNew {
		return synclib.ActionCreated
	}

	if eventCount == 1 {
		return synclib.ActionUpdated
	}

	return synclib.ActionUnchanged
}

func (s *CQRSStack) Count(ctx context.Context) (int64, error) {
	return s.ReadModel.Count(ctx, provider.ItemFilter{
		Type:       nil,
		ActorLogin: nil,
		RepoName:   nil,
		Source:     nil,
		Since:      nil,
		Limit:      0,
		Offset:     0,
	})
}

func (s *CQRSStack) GetTypes(ctx context.Context) ([]string, error) {
	return s.ReadModel.GetTypes(ctx)
}

func (s *CQRSStack) ListItems(
	ctx context.Context,
	filter provider.ItemFilter,
) ([]*provider.Item, error) {
	return s.ReadModel.List(ctx, filter)
}

func (s *CQRSStack) CountItems(
	ctx context.Context,
	filter provider.ItemFilter,
) (int64, error) {
	return s.ReadModel.Count(ctx, filter)
}

func (s *CQRSStack) GetItemTypes(ctx context.Context) ([]string, error) {
	return s.ReadModel.GetTypes(ctx)
}

func (s *CQRSStack) Close() error {
	var errs []error

	if s.CommandDispatcher != nil {
		if err := s.CommandDispatcher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.QueryDispatcher != nil {
		if err := s.QueryDispatcher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.outboxPublisher != nil {
		if err := s.outboxPublisher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.cancelRunner != nil {
		s.cancelRunner()
	}

	if err := s.ReadModel.Close(); err != nil {
		errs = append(errs, err)
	}

	if s.outbox != nil {
		if err := s.outbox.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := s.Store.Close(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
