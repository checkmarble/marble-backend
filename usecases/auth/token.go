package auth

import (
	"context"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type TokenHandler interface {
	GetToken(ctx context.Context, r *http.Request) (Token, error)
}

type MarbleTokenHandler struct {
	extractor Extractor
	verifier  Verifier
	generator TokenGenerator
}

func NewTokenHandler(e Extractor, v Verifier, g TokenGenerator) TokenHandler {
	return MarbleTokenHandler{extractor: e, verifier: v, generator: g}
}

func (h MarbleTokenHandler) GetToken(ctx context.Context, r *http.Request) (Token, error) {
	c, err := h.extractor.Extract(r)
	if err != nil {
		return Token{}, err
	}
	organizationID := uuid.Nil
	if r != nil && r.URL != nil && r.URL.Query().Get("organization_id") != "" {
		requestedOrganizationID := r.URL.Query().Get("organization_id")
		organizationID, err = uuid.Parse(requestedOrganizationID)
		if err != nil {
			return Token{}, models.BadParameterError
		}
	}

	intoCredentials, claims, err := h.verifier.Verify(ctx, c)
	if err != nil {
		return Token{}, err
	}

	token, err := h.generator.GenerateToken(ctx, c, intoCredentials, claims, organizationID)
	if err != nil {
		return Token{}, err
	}

	return token, nil
}
