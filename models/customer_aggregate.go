package models

import (
	"slices"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
)

type CustomerAggregateType string

const (
	CustomerAggregateTypePeriod    CustomerAggregateType = "period"
	CustomerAggregateTypeHistogram CustomerAggregateType = "histogram"
	CustomerAggregateTypePie       CustomerAggregateType = "pie"
)

var ValidCustomerAggregateTypes = []CustomerAggregateType{CustomerAggregateTypePeriod, CustomerAggregateTypeHistogram, CustomerAggregateTypePie}

func IsValidCustomerAggregateType(kind CustomerAggregateType) bool {
	return slices.Contains(ValidCustomerAggregateTypes, kind)
}

type CustomerAggregate struct {
	Id         uuid.UUID
	OrgId      uuid.UUID
	TableId    uuid.UUID
	Name       string
	Type       CustomerAggregateType
	Expression ast.Node
	TimeSlice  string

	Results []CustomerAggregateResult
}

type CreateCustomerAggregate struct {
	Name       string
	Type       CustomerAggregateType
	RecordType string
	Expression ast.Node

	DryRun     bool
	CustomerId string
	TimeSlice  string
}

type UpdateCustomerAggregate struct {
	Id uuid.UUID

	CreateCustomerAggregate
}

func (c CreateCustomerAggregate) ToAggregateRequest() CustomerAggregateRequest {
	return CustomerAggregateRequest{
		Type:       c.Type,
		Expression: c.Expression,
		CustomerId: c.CustomerId,
		TimeSlice:  c.TimeSlice,
	}
}

type CustomerAggregateRequest struct {
	Type       CustomerAggregateType
	Expression ast.Node
	CustomerId string
	TimeSlice  string
}

type CustomerAggregateResult struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}
