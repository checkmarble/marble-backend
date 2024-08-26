package infra

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/golang-jwt/jwt/v4"
)

type Metabase struct {
	config MetabaseConfiguration
}

func InitializeMetabase(config MetabaseConfiguration) Metabase {
	return Metabase{
		config: config,
	}
}

func (metabase Metabase) GenerateSignedEmbeddingURL(analyticsCustomClaims models.AnalyticsCustomClaims) (string, error) {
	embeddingType := analyticsCustomClaims.GetEmbeddingType()
	resourceType := embeddingType.ResourceType()

	type Claims struct {
		Resource map[string]interface{} `json:"resource"`
		Params   map[string]interface{} `json:"params"`
		jwt.RegisteredClaims
	}
	claims := Claims{
		Resource: map[string]interface{}{
			resourceType: metabase.config.Resources[embeddingType],
		},
		Params: analyticsCustomClaims.GetParams(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute *
				time.Duration(metabase.config.TokenLifetimeMinute))),
			Issuer: "marble",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(metabase.config.JwtSigningKey)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/embed/%s/%s#theme=light&bordered=false&titled=false",
		metabase.config.SiteUrl, resourceType, signedToken), nil
}
