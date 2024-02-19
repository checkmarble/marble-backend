package ast_eval

import "github.com/checkmarble/marble-backend/models"

type EvaluationEnvironmentFactoryParams struct {
	OrganizationId                string
	Payload                       models.ClientObject
	DataModel                     models.DataModel
	DatabaseAccessReturnFakeValue bool
}

type AstEvaluationEnvironmentFactory func(params EvaluationEnvironmentFactoryParams) AstEvaluationEnvironment
