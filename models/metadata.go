package models

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type MetadataKey string

const (
	MetadataKeyDeploymentID MetadataKey = "deployment_id"
)

type Metadata struct {
	ID        uuid.UUID
	CreatedAt time.Time
	OrgID     *uuid.UUID
	Key       MetadataKey
	Value     string
}

func MetadataKeyFromString(key string) (MetadataKey, error) {
	switch key {
	case "deployment_id":
		return MetadataKeyDeploymentID, nil
	default:
		return "", errors.Newf("invalid metadata key: %s", key)
	}
}
