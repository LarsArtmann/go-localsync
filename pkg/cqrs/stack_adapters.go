// stack_adapters.go bridges CQRSStack to synclib.SyncStore.
//
// The read-side methods (List / Count / CountByType) call the ReadModel
// directly. The ReadModel IS the read side of this CQRS stack: queries are
// allocation-free point-to-point calls, so there is no query dispatcher to
// route through. The command side remains dispatched (see stack.go:
// wireCommandDispatcher) for logging, retry, and validation middleware.

package cqrs

import (
	"context"
	"errors"
	"io"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

func classifyAction(err error, outcome SyncOutcome) synclib.SyncAction {
	if err != nil {
		return synclib.ActionError
	}

	if outcome.ConflictDetected {
		if outcome.ConflictWinner == ConflictWinnerLocal {
			return synclib.ActionConflictLocal
		}

		return synclib.ActionConflictRemote
	}

	if outcome.EventCount == 1 && outcome.WasNew {
		return synclib.ActionCreated
	}

	if outcome.EventCount == 1 {
		return synclib.ActionUpdated
	}

	return synclib.ActionUnchanged
}

func (s *CQRSStack) List(ctx context.Context, filter model.ItemFilter) ([]*model.Item, error) {
	return s.ReadModel.List(ctx, filter)
}

func (s *CQRSStack) Count(ctx context.Context, filter model.ItemFilter) (int64, error) {
	return s.ReadModel.Count(ctx, filter)
}

func (s *CQRSStack) CountByType(ctx context.Context, filter model.ItemFilter) (map[string]int64, error) {
	return s.ReadModel.CountByType(ctx, filter)
}

func (s *CQRSStack) Close() error {
	var errs []error

	if s.CommandDispatcher != nil {
		if err := s.CommandDispatcher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.cancelRunner != nil {
		s.cancelRunner()
	}

	if s.ReadModel != nil {
		if err := s.ReadModel.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// v3 removed io.Closer from event.Store/event.Bus interfaces (ADR-0010).
	// The concrete stores and the watermill EventBus still implement Close();
	// type-assert to invoke it where present.
	if s.Store != nil {
		if c, ok := s.Store.(io.Closer); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if s.Bus != nil {
		if c, ok := s.Bus.(io.Closer); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
