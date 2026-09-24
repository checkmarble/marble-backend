package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) ListUserRoleBindings(ctx context.Context, exec Executor, userId string) ([]models.RoleBinding, error) {
	return repo.listRoleBindings(ctx, exec, squirrel.Eq{"rb.user_id": userId})
}

func (repo *MarbleDbRepository) ListApiKeyRoleBindings(ctx context.Context, exec Executor, apiKeyId string) ([]models.RoleBinding, error) {
	return repo.listRoleBindings(ctx, exec, squirrel.Eq{"rb.api_key_id": apiKeyId})
}

func (repo *MarbleDbRepository) ReplaceUserRoleBindings(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	userId string,
	bindings []models.RoleBinding,
) error {
	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Delete(dbmodels.TABLE_ROLE_BINDINGS).
			Where(squirrel.Eq{"user_id": userId}),
	)

	if err != nil {
		return err
	}

	return repo.insertRoleBindings(ctx, exec, orgId, &userId, nil, bindings)
}

func (repo *MarbleDbRepository) ReplaceApiKeyRoleBindings(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	apiKeyId string,
	bindings []models.RoleBinding,
) error {
	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Delete(dbmodels.TABLE_ROLE_BINDINGS).
			Where(squirrel.Eq{"api_key_id": apiKeyId}),
	)

	if err != nil {
		return err
	}

	return repo.insertRoleBindings(ctx, exec, orgId, nil, &apiKeyId, bindings)
}

var roleBindingSelectColumns = []string{
	"rb.id",
	"rb.org_id",
	"rb.user_id",
	"rb.api_key_id",
	"rb.native_role",
	"rb.custom_role_id",
	"rb.conditions",
	"r.slug as custom_role_slug",
	"coalesce(r.permissions, array[]::text[]) as custom_permissions",
}

func (repo *MarbleDbRepository) listRoleBindings(
	ctx context.Context,
	exec Executor,
	where squirrel.Eq,
) ([]models.RoleBinding, error) {
	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(roleBindingSelectColumns...).
			From(dbmodels.TABLE_ROLE_BINDINGS+" rb").
			LeftJoin(dbmodels.TABLE_ROLES+" r on r.id = rb.custom_role_id and r.org_id = rb.org_id").
			Where(where).
			OrderBy("rb.id"),
		dbmodels.AdaptRoleBindingModel,
	)
}

func (repo *MarbleDbRepository) insertRoleBindings(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	userId, apiKeyId *string,
	bindings []models.RoleBinding,
) error {
	var persistedOrgId any = orgId

	if orgId == uuid.Nil {
		persistedOrgId = nil
	}

	for _, binding := range bindings {
		if binding.Role == "" {
			return fmt.Errorf("role binding must reference a role: %w", models.BadParameterError)
		}

		if err := binding.Conditions.Validate(); err != nil {
			return err
		}

		id := binding.Id

		if id == uuid.Nil {
			id = pure_utils.NewId()
		}

		var nativeRole *string

		if binding.Role.IsCustom() {
			if binding.CustomRoleId == nil {
				return fmt.Errorf("custom role binding must be resolved: %w", models.BadParameterError)
			}
		} else {
			if binding.CustomRoleId != nil {
				return fmt.Errorf("native role binding cannot reference a custom role: %w", models.BadParameterError)
			}
			value := string(binding.Role)
			nativeRole = &value
		}

		conditions, err := json.Marshal(binding.Conditions)
		if err != nil {
			return err
		}

		if err := ExecBuilder(
			ctx,
			exec,
			NewQueryBuilder().
				Insert(dbmodels.TABLE_ROLE_BINDINGS).
				Columns(
					"id",
					"org_id",
					"user_id",
					"api_key_id",
					"native_role",
					"custom_role_id",
					"conditions",
				).
				Values(
					id,
					persistedOrgId,
					userId,
					apiKeyId,
					nativeRole,
					binding.CustomRoleId,
					squirrel.Expr("?::jsonb", conditions),
				),
		); err != nil {
			return err
		}
	}
	return nil
}
