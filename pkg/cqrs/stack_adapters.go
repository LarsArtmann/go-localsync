// stack_adapters.go bridges CQRSStack to synclib.SyncStore.
//
// The List / Count / GetTypes methods below deliberately bypass the
// QueryDispatcher and call the ReadModel directly. Reasons:
//
//  1. Hot path performance: these read endpoints are called for every
//     GET /items, GET /stats, and every SyncItems result. Routing them
//     through the dispatcher would add reflection / middleware overhead
//     with no behavioral benefit at the call sites.
//
//  2. The query.Dispatcher is still wired (see stack.go:wireQueryDispatcher)
//     and registered in tests for verifying handler resolution and
//     middleware behavior. It is intentionally not used at runtime.
//
//  3. The dispatcher's `query.ListItemsHandler` (queries.go) remains the
//     single source of truth for the read-side query contract — these
//     adapters just happen to be the direct implementation it would
//     delegate to.
//
// If you are adding a new read endpoint, prefer extending this adapter
// file over creating a new dispatcher handler unless the endpoint needs
// the cross-cutting middleware chain (logging, metrics, retry).

package cqrs

import (
	"context"
	"errors"
	"io"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

func classifyAction(err error, eventCount int, wasNew bool, conflictWinner ConflictWinner) synclib.SyncAction {
	if err != nil {
		return synclib.ActionError
	}

	if eventCount > 1 {
		if conflictWinner == ConflictWinnerLocal {
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

func (s *CQRSStack) List(ctx context.Context, filter model.ItemFilter) ([]*model.Item, error) {
	return s.ReadModel.List(ctx, filter)
}

func (s *CQRSStack) Count(ctx context.Context, filter model.ItemFilter) (int64, error) {
	return s.ReadModel.Count(ctx, filter)
}

func (s *CQRSStack) CountByType(ctx context.Context, filter model.ItemFilter) (map[string]int64, error) {
	return s.ReadModel.CountByType(ctx, filter)
}

func (s *CQRSStack) GetTypes(ctx context.Context) ([]string, error) {
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
