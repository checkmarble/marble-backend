package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type failingClientIndexExecutor struct{ Executor }

func (failingClientIndexExecutor) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{Schema: "client"}
}
func (failingClientIndexExecutor) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("client connection closed")
}
func TestClientDatabaseIndexQueryErrorIsClassified(t *testing.T) {
	_, err := (&ClientDbRepository{}).listAllPgIndexes(context.Background(), failingClientIndexExecutor{})
	require.Error(t, err)
	var clientError ClientDatabaseError
	require.ErrorAs(t, err, &clientError)
}
