package repositories

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUpdateSarDeclarationDocument_SQLChecksVersion(t *testing.T) {
	id := uuid.New()
	expectedVersion := 3
	document := []byte(`{"a":1}`)

	query := updateSarDeclarationDocumentQuery(id, expectedVersion, document, "analysis", "extra")

	sql, args, err := query.ToSql()
	require.NoError(t, err)
	require.Contains(t, sql, "UPDATE sar_declarations")
	require.Contains(t, sql, "version = version + 1")
	require.Contains(t, sql, "version = $")
	require.Contains(t, sql, "id = $")
	require.Contains(t, args, id)
	require.Contains(t, args, expectedVersion)
}
