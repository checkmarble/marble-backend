package usecases

import (
	"context"
	"net/http"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// This is a usecase because, ultimately, it will need to use repositories to retrieve information.
type OidcUsecase struct{}

func (uc OidcUsecase) ExchangeToken(ctx context.Context, cfg infra.OidcConfig, r *http.Request) (*oauth2.Token, error) {
	if err := r.ParseForm(); err != nil {
		return nil, errors.Wrap(err, "could not read form data")
	}

	f := r.Form

	req := oauth2.Config{
		Endpoint:     cfg.Provider.Endpoint(),
		ClientID:     cfg.ClientId,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectUri,
		Scopes:       cfg.Scopes,
	}

	switch f.Get("grant_type") {
	case "authorization_code":
		return req.Exchange(ctx, f.Get("code"), oauth2.VerifierOption(f.Get("code_verifier")))

	case "refresh_token":
		src := req.TokenSource(ctx, &oauth2.Token{RefreshToken: f.Get("refresh_token")})

		tokens, err := src.Token()
		if err != nil {
			return nil, err
		}

		if tokens.Extra("id_token") == "" {
			return nil, errors.New("ID token was not reissued during refresh")
		}

		return tokens, nil
	}

	return nil, errors.New("invalid grant type")
}
