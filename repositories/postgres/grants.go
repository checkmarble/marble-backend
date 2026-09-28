package postgres

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

// ActiveGrantsForPrincipal returns every currently active grant of a principal,
// across all scopes, with custom role permissions resolved.
func (db *Database) ActiveGrantsForPrincipal(ctx context.Context, principalType, principalID string) ([]models.RoleBinding, error) {
	query, args, err := NewQueryBuilder().
		Select(dbmodels.SelectRoleBindingColumns...).
		From("active_grants g").
		LeftJoin(dbmodels.TABLE_ROLES+" r on r.id = g.custom_role_id and r.org_id = g.organization_id").
		Where(squirrel.Eq{
			"g.principal_type":      principalType,
			"g.principal_id":        principalID,
			"g.principal_authority": dbmodels.GrantAuthorityMarble,
		}).
		OrderBy("g.created_at", "g.id").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	dbBindings, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbmodels.DbRoleBinding])
	if err != nil {
		return nil, err
	}

	bindings := make([]models.RoleBinding, 0, len(dbBindings))
	for _, binding := range dbBindings {
		adapted, err := dbmodels.AdaptRoleBinding(binding)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, adapted)
	}

	return bindings, nil
}
