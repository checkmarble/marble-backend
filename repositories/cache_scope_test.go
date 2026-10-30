package repositories

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

type cacheScopeExecutor struct {
	pgxmock.PgxPoolIface
	client *RedisClient
}

func (e cacheScopeExecutor) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{SchemaType: models.DATABASE_SCHEMA_TYPE_MARBLE}
}
func (e cacheScopeExecutor) Cache(ctx context.Context) *RedisExecutor {
	creds, _ := utils.CredentialsFromCtx(ctx)
	return e.client.NewExecutor(creds.OrganizationId)
}
func (e cacheScopeExecutor) Begin(context.Context) (Transaction, error) {
	panic("unexpected transaction")
}
func newCacheScopeTest(t *testing.T) (*MarbleDbRepository, cacheScopeExecutor, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client, err := NewRedisClient(infra.RedisConfig{Address: server.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.client.Close() })
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.ExpectationsWereMet()); pool.Close() })
	repo := NewRepositories(nil, infra.GcpConfig{}, WithRedisClient(client)).MarbleDbRepository
	return repo, cacheScopeExecutor{PgxPoolIface: pool, client: client}, server
}
func cacheJSON(t *testing.T, server *miniredis.Miniredis, key string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, server.Set(key, string(data)))
}
func TestDataModelCacheUsesRequestedOrganization(t *testing.T) {
	repo, exec, server := newCacheScopeTest(t)
	org, other := uuid.New(), uuid.New()
	for _, enums := range []bool{false, true} {
		suffix := "data-model:" + strconv.FormatBool(enums)
		expected := models.DataModel{Tables: map[string]models.Table{"target": {Name: "target"}}}
		cacheJSON(t, server, org.String()+":"+suffix, expected)
		cacheJSON(t, server, other.String()+":"+suffix, models.DataModel{})
		cacheJSON(t, server, suffix, models.DataModel{})
		for _, ctx := range []context.Context{context.Background(), cacheContext(other)} {
			got, err := repo.GetDataModel(ctx, exec, org, enums, true)
			require.NoError(t, err)
			require.Len(t, got.Tables, 1)
			require.Equal(t, "target", got.Tables["target"].Name)
		}
	}
}
func TestDataModelCacheWriteUsesRequestedOrganization(t *testing.T) {
	repo, exec, server := newCacheScopeTest(t)
	org, other := uuid.New(), uuid.New()
	ctx := cacheContext(other)
	exec.ExpectQuery("SELECT").WithArgs(org.String()).WillReturnRows(pgxmock.NewRows(nil))
	exec.ExpectQuery("SELECT").WithArgs(org).WillReturnRows(pgxmock.NewRows(nil))
	_, err := repo.GetDataModel(ctx, exec, org, false, false)
	require.NoError(t, err)
	require.True(t, server.Exists(org.String()+":data-model:false"))
	require.False(t, server.Exists(other.String()+":data-model:false"))
	require.False(t, server.Exists("data-model:false"))
}
func TestDataModelCreateInvalidatesRequestedOrganization(t *testing.T) {
	repo, exec, server := newCacheScopeTest(t)
	org, other := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{org, other} {
		for _, enums := range []string{"true", "false"} {
			require.NoError(t, server.Set(id.String()+":data-model:"+enums, "{}"))
		}
	}
	exec.ExpectExec("INSERT INTO data_model_tables").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := repo.CreateDataModelTable(cacheContext(other), exec, org, uuid.NewString(), models.CreateTableInput{Name: "accounts"})
	require.NoError(t, err)
	for _, enums := range []string{"true", "false"} {
		require.False(t, server.Exists(org.String()+":data-model:"+enums))
		require.True(t, server.Exists(other.String()+":data-model:"+enums))
	}
}
func TestUserCacheIsGlobal(t *testing.T) {
	repo, exec, server := newCacheScopeTest(t)
	user := models.User{UserId: models.UserId(uuid.NewString()), FirstName: "current"}
	cacheJSON(t, server, "user:"+string(user.UserId), user)
	for _, org := range []uuid.UUID{uuid.New(), uuid.New()} {
		cacheJSON(t, server, org.String()+":user:"+string(user.UserId), models.User{UserId: user.UserId, FirstName: "stale"})
		got, err := repo.UserById(cacheContext(org), exec, string(user.UserId))
		require.NoError(t, err)
		require.Equal(t, user, got)
	}
}
func TestUserMutationsInvalidateGlobalCache(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(strconv.FormatBool(remove), func(t *testing.T) {
			repo, exec, server := newCacheScopeTest(t)
			id := uuid.NewString()
			cacheJSON(t, server, "user:"+id, models.User{UserId: models.UserId(id)})
			ctx := cacheContext(uuid.New())
			if remove {
				exec.ExpectExec("UPDATE users").WithArgs(models.UserId(id)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				exec.ExpectExec("UPDATE grants").WithArgs(models.UserId(id), "user").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				require.NoError(t, repo.DeleteUser(ctx, exec, models.UserId(id)))
			} else {
				exec.ExpectExec("UPDATE users").WithArgs("new name", id).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				require.NoError(t, repo.UpdateUser(ctx, exec, models.UpdateUser{UserId: id, FirstName: utils.Ptr("new name")}))
			}
			require.False(t, server.Exists("user:"+id))
		})
	}
}

func TestDataModelMutationInvalidatesOwner(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		args        []any
		mutate      func(*MarbleDbRepository, context.Context, Executor, string) error
	}{
		{"table", "UPDATE data_model_tables .* RETURNING organization_id", []any{"new description", "id"}, func(r *MarbleDbRepository, ctx context.Context, e Executor, id string) error {
			return r.UpdateDataModelTable(ctx, e, id, utils.Ptr("new description"), pure_utils.Null[models.FollowTheMoneyEntity]{}, pure_utils.Null[string]{}, pure_utils.Null[models.SemanticType]{}, pure_utils.Null[string]{}, pure_utils.Null[string]{}, nil, nil)
		}},
		{"field", "UPDATE data_model_fields .* RETURNING .*organization_id", []any{"new description", "id"}, func(r *MarbleDbRepository, ctx context.Context, e Executor, id string) error {
			return r.UpdateDataModelField(ctx, e, id, models.UpdateFieldInput{Description: utils.Ptr("new description")})
		}},
		{"link", "DELETE FROM data_model_links .* RETURNING organization_id", []any{"id"}, func(r *MarbleDbRepository, ctx context.Context, e Executor, id string) error {
			return r.DeleteDataModelLink(ctx, e, id)
		}},
		{"pivot", "DELETE FROM data_model_pivots .* RETURNING organization_id", []any{"id"}, func(r *MarbleDbRepository, ctx context.Context, e Executor, id string) error {
			return r.DeleteDataModelPivot(ctx, e, id)
		}},
		{"soft delete pivot", "UPDATE data_model_pivots .* RETURNING organization_id", []any{"id"}, func(r *MarbleDbRepository, ctx context.Context, e Executor, id string) error {
			return r.SoftDeletePivot(ctx, e, id)
		}},
		{"restore pivot", "UPDATE data_model_pivots .* RETURNING organization_id", []any{nil, "id"}, func(r *MarbleDbRepository, ctx context.Context, e Executor, id string) error {
			return r.RestorePivot(ctx, e, id)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, exec, server := newCacheScopeTest(t)
			owner, caller := uuid.New(), uuid.New()
			for _, org := range []uuid.UUID{owner, caller} {
				for _, enums := range []string{"true", "false"} {
					require.NoError(t, server.Set(org.String()+":data-model:"+enums, "{}"))
				}
			}
			exec.ExpectQuery(tc.query).WithArgs(tc.args...).WillReturnRows(pgxmock.NewRows([]string{"organization_id"}).AddRow(owner.String()))
			require.NoError(t, tc.mutate(repo, cacheContext(caller), exec, "id"))
			for _, enums := range []string{"true", "false"} {
				require.False(t, server.Exists(owner.String()+":data-model:"+enums))
				require.True(t, server.Exists(caller.String()+":data-model:"+enums))
			}
		})
	}
}

func cacheContext(org uuid.UUID) context.Context {
	return context.WithValue(context.Background(), utils.ContextKeyCredentials, models.Credentials{OrganizationId: org})
}

func TestUserCacheWriteIsSharedAcrossOrganizations(t *testing.T) {
	repo, exec, server := newCacheScopeTest(t)
	id := uuid.NewString()
	orgA, orgB := uuid.New(), uuid.New()
	exec.ExpectQuery("SELECT .* FROM users").WithArgs(id).WillReturnRows(
		pgxmock.NewRows([]string{"id", "email", "role", "organization_id", "first_name", "last_name", "deleted_at", "ai_assist_enabled", "picture"}).
			AddRow(id, "user@example.com", int(models.VIEWER), nil, "current", "name", nil, false, ""))
	first, err := repo.UserById(cacheContext(orgA), exec, id)
	require.NoError(t, err)
	second, err := repo.UserById(cacheContext(orgB), exec, id)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.True(t, server.Exists("user:"+id))
	require.False(t, server.Exists(orgA.String()+":user:"+id))
	require.False(t, server.Exists(orgB.String()+":user:"+id))
}

func TestEnumUpdateInvalidatesRequestedOrganization(t *testing.T) {
	repo, exec, server := newCacheScopeTest(t)
	org := uuid.New()
	key := org.String() + ":data-model:true"
	require.NoError(t, server.Set(key, "{}"))
	fieldID := uuid.NewString()
	exec.ExpectExec("INSERT INTO data_model_enum_values").WithArgs(fieldID, "active").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := repo.BatchInsertEnumValues(context.Background(), exec, org,
		models.EnumValues{"status": {"active": {}}},
		models.Table{Fields: map[string]models.Field{"status": {ID: fieldID, DataType: models.String}}})
	require.NoError(t, err)
	require.False(t, server.Exists(key))
}

func TestMissingDataModelRowKeepsCache(t *testing.T) {
	repo, exec, server := newCacheScopeTest(t)
	ctx := cacheContext(uuid.New())
	key := "data-model:false"
	require.NoError(t, server.Set(key, "{}"))
	exec.ExpectQuery("DELETE FROM data_model_links .* RETURNING organization_id").WithArgs("missing").WillReturnRows(pgxmock.NewRows([]string{"organization_id"}))
	require.NoError(t, repo.DeleteDataModelLink(ctx, exec, "missing"))
	require.True(t, server.Exists(key))
}
