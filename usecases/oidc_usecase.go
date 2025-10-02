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
	f.Set("client_secret", cfg.ClientSecret)

	req := oauth2.Config{
		Endpoint:     cfg.Provider.Endpoint(),
		ClientID:     cfg.ClientId,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectUri,
		Scopes:       cfg.Scopes,
	}

	return req.Exchange(ctx, f.Get("code"), oauth2.VerifierOption(f.Get("code_verifier")))
}
