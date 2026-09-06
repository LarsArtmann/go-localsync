package cqrs

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

type scannedItem struct {
	itemIDStr, source, sourceID, eventType string
	attributesJSON                         string
	contentHash, tombstoneReason           string
	createdAt, updatedAt                   time.Time
	tombstoned, schemaVersion              int
	tombstonedAt                           sql.NullTime
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

	var attrs map[string]string
	if si.attributesJSON != "" && si.attributesJSON != "{}" {
		if err := json.Unmarshal([]byte(si.attributesJSON), &attrs); err != nil {
			return nil, pkgerrors.Wrap(err, "unmarshal attributes from row")
		}
	}

	return &model.Item{
		ID:            itemID,
		SourceID:      id.NewSourceID(si.sourceID),
		Source:        id.NewProviderID(si.source),
		Type:          id.NewEventTypeID(si.eventType),
		Attributes:    attrs,
		ContentHash:   id.NewContentHash(si.contentHash),
		Tombstone:     tombstone,
		CreatedAt:     si.createdAt,
		UpdatedAt:     si.updatedAt,
		SchemaVersion: schema.Version(si.schemaVersion),
	}, nil
}

func newScannedItem() *scannedItem {
	return &scannedItem{
		itemIDStr:       "",
		source:          "",
		sourceID:        "",
		eventType:       "",
		attributesJSON:  "{}",
		contentHash:     "",
		tombstoneReason: "",
		createdAt:       time.Time{},
		updatedAt:       time.Time{},
		tombstoned:      0,
		tombstonedAt:    sql.NullTime{},
		schemaVersion:   0,
	}
}

func scanItem(row *sql.Row) (*model.Item, error) {
	si := newScannedItem()

	err := row.Scan(&si.itemIDStr, &si.source, &si.sourceID, &si.eventType, &si.attributesJSON,
		&si.createdAt, &si.updatedAt, &si.tombstoned, &si.tombstoneReason, &si.tombstonedAt,
		&si.contentHash, &si.schemaVersion)
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
			&si.attributesJSON,
			&si.createdAt,
			&si.updatedAt,
			&si.tombstoned,
			&si.tombstoneReason,
			&si.tombstonedAt,
			&si.contentHash,
			&si.schemaVersion,
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
