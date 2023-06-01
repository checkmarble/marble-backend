package usecases

import "marble/marble-backend/repositories"

type OperatorEvalUsecase struct {
	dbPoolRepository           repositories.DbPoolRepository
	ingestedDataReadRepository repositories.IngestedDataReadRepository
}
