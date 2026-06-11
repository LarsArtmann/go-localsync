package cqrs

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
)

type scannedItem struct {
	itemIDStr, source, sourceID, eventType, actorLogin, actorAvatarURL, repoName, repoURL string
	createdAt, updatedAt                                                                  time.Time
}

//nolint:exhaustruct // SchemaVersion not stored in read model schema
func (si *scannedItem) toItem() (*model.Item, error) {
	itemID, err := parseItemID(si.itemIDStr)
	if err != nil {
		return nil, fmt.Errorf("parse item ID from row: %w", err)
	}

	return &model.Item{
		ID:             itemID,
		ExternalID:     id.NewExternalID(si.sourceID),
		Source:         id.NewProviderID(si.source),
		Type:           id.NewEventTypeID(si.eventType),
		ActorLogin:     id.NewActorID(si.actorLogin),
		ActorAvatarURL: si.actorAvatarURL,
		RepoName:       id.NewRepoID(si.repoName),
		RepoURL:        si.repoURL,
		CreatedAt:      si.createdAt,
		UpdatedAt:      si.updatedAt,
	}, nil
}

func newScannedItem() *scannedItem {
	return &scannedItem{
		itemIDStr:      "",
		source:         "",
		sourceID:       "",
		eventType:      "",
		actorLogin:     "",
		actorAvatarURL: "",
		repoName:       "",
		repoURL:        "",
		createdAt:      time.Time{},
		updatedAt:      time.Time{},
	}
}

func scanItem(row *sql.Row) (*model.Item, error) {
	si := newScannedItem()

	err := row.Scan(&si.itemIDStr, &si.source, &si.sourceID, &si.eventType, &si.actorLogin,
		&si.actorAvatarURL, &si.repoName, &si.repoURL, &si.createdAt, &si.updatedAt)
	if err != nil {
		return nil, err
	}

	return si.toItem()
}

func scanItems(rows *sql.Rows) ([]*model.Item, error) {
	var items []*model.Item

	for rows.Next() {
		si := newScannedItem()

		err := rows.Scan(
			&si.itemIDStr,
			&si.source,
			&si.sourceID,
			&si.eventType,
			&si.actorLogin,
			&si.actorAvatarURL,
			&si.repoName,
			&si.repoURL,
			&si.createdAt,
			&si.updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}

		item, err := si.toItem()
		if err != nil {
			return nil, fmt.Errorf("convert row to item: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}

	return items, nil
}
