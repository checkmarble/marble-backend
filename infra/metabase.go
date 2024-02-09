package infra

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/golang-jwt/jwt/v4"
)

type Metabase struct {
	config models.MetabaseConfiguration
}

func InitializeMetabase(config models.MetabaseConfiguration) Metabase {
	return Metabase{
		config: config,
	}
}

func (metabase Metabase) GenerateSignedEmbeddingURL(analyticsCustomClaims models.AnalyticsCustomClaims) (string, error) {
	claims := struct {
		Resource map[string]interface{} `json:"resource"`
		Params   map[string]interface{} `json:"params"`
		jwt.RegisteredClaims
	}{
		Resource: analyticsCustomClaims.Resource,
		Params:   analyticsCustomClaims.Params,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(metabase.config.TokenLifetimeMinute))),
			Issuer:    "marble",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(metabase.config.JwtSigningKey))
	if err != nil {
		return "", err
	}

	ressource := ""
	if _, found := analyticsCustomClaims.Resource["dashboard"]; found {
		ressource = "dashboard"
	}
	if _, found := analyticsCustomClaims.Resource["question"]; found {
		ressource = "question"
	}
	if ressource == "" {
		return "", fmt.Errorf("resource not found in analytics custom claims")
	}

	return fmt.Sprintf("%s/embed/%s/%s#theme=light&bordered=false&titled=false", metabase.config.SiteUrl, ressource, signedToken), nil
}
