package ast_eval

import "marble/marble-backend/models"

type EvaluationEnvironmentFactoryParams struct {
	OrganizationId                string
	Payload                       models.PayloadReader
	DataModel                     models.DataModel
	DatabaseAccessReturnFakeValue bool
}

type AstEvaluationEnvironmentFactory func(params EvaluationEnvironmentFactoryParams) AstEvaluationEnvironment
