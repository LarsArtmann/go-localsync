package cqrs

import (
	"database/sql"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

type scannedItem struct {
	itemIDStr, source, sourceID, eventType, actorLogin, actorAvatarURL, repoName, repoURL string
	tombstoneReason                                                                       string
	createdAt, updatedAt                                                                  time.Time
	tombstoned                                                                            int
	tombstonedAt                                                                          sql.NullTime
}

func (si *scannedItem) toItem() (*model.Item, error) {
	itemID, err := parseItemID(si.itemIDStr)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "parse item ID from row")
	}

	var tombstone model.Tombstone
	if si.tombstoned != 0 {
		tombstone = model.Tombstone{Reason: model.ParseTombstoneReason(si.tombstoneReason)}
		if si.tombstonedAt.Valid {
			tombstone.At = si.tombstonedAt.Time
		}
	}

	return &model.Item{
		ID:             itemID,
		ExternalID:     id.NewExternalID(si.sourceID),
		Source:         id.NewProviderID(si.source),
		Type:           id.NewEventTypeID(si.eventType),
		ActorLogin:     id.NewActorLogin(si.actorLogin),
		ActorAvatarURL: si.actorAvatarURL,
		RepoName:       id.NewRepoID(si.repoName),
		RepoURL:        si.repoURL,
		Tombstone:      tombstone,
		CreatedAt:      si.createdAt,
		UpdatedAt:      si.updatedAt,
	}, nil
}

func newScannedItem() *scannedItem {
	return &scannedItem{
		itemIDStr:       "",
		source:          "",
		sourceID:        "",
		eventType:       "",
		actorLogin:      "",
		actorAvatarURL:  "",
		repoName:        "",
		repoURL:         "",
		tombstoneReason: "",
		createdAt:       time.Time{},
		updatedAt:       time.Time{},
		tombstoned:      0,
		tombstonedAt:    sql.NullTime{},
	}
}

func scanItem(row *sql.Row) (*model.Item, error) {
	si := newScannedItem()

	err := row.Scan(&si.itemIDStr, &si.source, &si.sourceID, &si.eventType, &si.actorLogin,
		&si.actorAvatarURL, &si.repoName, &si.repoURL, &si.createdAt, &si.updatedAt,
		&si.tombstoned, &si.tombstoneReason, &si.tombstonedAt)
	if err != nil {
		return nil, err
	}

	return si.toItem()
}

const defaultScanItemsCapacity = 64

func scanItems(rows *sql.Rows) ([]*model.Item, error) {
	items := make([]*model.Item, 0, defaultScanItemsCapacity)
	si := newScannedItem()

	for rows.Next() {
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
			&si.tombstoned,
			&si.tombstoneReason,
			&si.tombstonedAt,
		)
		if err != nil {
			return nil, pkgerrors.Wrap(err, "scan item")
		}

		item, err := si.toItem()
		if err != nil {
			return nil, pkgerrors.Wrap(err, "convert row to item")
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, pkgerrors.Wrap(err, "iterate items")
	}

	return items, nil
}
