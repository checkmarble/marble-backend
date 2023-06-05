package pg_repository

import (
	"context"
	"errors"
	"fmt"

	"marble/marble-backend/app"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dbToken struct {
	ID        string      `db:"id"`
	OrgID     string      `db:"org_id"`
	Token     string      `db:"token"`
	DeletedAt pgtype.Time `db:"deleted_at"`
}

func (t *dbToken) toDomain() app.Token {
	return app.Token{
		ID:    t.ID,
		OrgID: t.OrgID,
		Token: t.Token,
	}
}

func (r *PGRepository) GetOrganizationIDFromApiKey(ctx context.Context, apiKey string) (orgID string, err error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("tokens t").
		Where("token = ?", apiKey).
		ToSql()
	if err != nil {
		return "", fmt.Errorf("unable to build apiKey query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	token, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbToken])
	if errors.Is(err, pgx.ErrNoRows) {
		return "", app.ErrNotFoundInRepository
	} else if err != nil {
		return "", fmt.Errorf("unable to get org from apiKey %s: %w", apiKey, err)
	}

	return token.OrgID, nil
}

func (r *PGRepository) GetTokens(ctx context.Context, orgID string) (map[string]string, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("tokens").
		Where("org_id = ?", orgID).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build tokens query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	tokens, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbToken])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, app.ErrNotFoundInRepository
	} else if err != nil {
		return nil, fmt.Errorf("unable to get tokens for org(id: %s): %w", orgID, err)
	}

	var orgTokens map[string]string
	for _, token := range tokens {
		orgTokens[token.ID] = token.Token
	}
	return orgTokens, nil
}

type CreateToken struct {
	OrgID string
	Token string
}

func (r *PGRepository) CreateToken(ctx context.Context, token CreateToken) (app.Token, error) {
	sql, args, err := r.queryBuilder.
		Insert("tokens").
		Columns(
			"org_id",
			"token",
		).
		Values(
			token.OrgID,
			token.Token,
		).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.Token{}, fmt.Errorf("unable to build token query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	createdToken, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbToken])
	if err != nil {
		return app.Token{}, fmt.Errorf("unable to create token: %w", err)
	}

	return createdToken.toDomain(), nil
}
