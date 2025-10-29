package models

import "github.com/google/uuid"

type InsertScreeningMonitoringObject struct {
	TableName string
	ObjectId  string
	ConfigId  uuid.UUID
}
