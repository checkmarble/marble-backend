package ast_eval

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type EvaluationEnvironmentFactoryParams struct {
	OrganizationId                uuid.UUID
	ClientObject                  models.ClientObject
	DataModel                     models.DataModel
	DatabaseAccessReturnFakeValue bool
}

type AstEvaluationEnvironmentFactory func(params EvaluationEnvironmentFactoryParams) AstEvaluationEnvironment
