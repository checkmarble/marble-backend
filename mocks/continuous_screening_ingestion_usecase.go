package mocks

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/usecases/payload_parser"
)

type ContinuousScreeningIngestionUsecase struct {
	mock.Mock
}

func (m *ContinuousScreeningIngestionUsecase) IngestObject(
	ctx context.Context,
	organizationId string,
	objectType string,
	objectBody json.RawMessage,
	shouldScreen bool,
	parserOpts ...payload_parser.ParserOpt,
) (int, error) {
	args := m.Called(ctx, organizationId, objectType, objectBody, shouldScreen)
	return args.Int(0), args.Error(1)
}
