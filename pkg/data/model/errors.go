package model

import "errors"

var (
	errMissingExternalID = errors.New("provider item: externalID is required")
	errMissingSource     = errors.New("provider item: source is required")
	errMissingType       = errors.New("provider item: type is required")
	errMissingCreatedAt  = errors.New("provider item: createdAt is required")
	errMissingUpdatedAt  = errors.New("provider item: updatedAt is required")
)
