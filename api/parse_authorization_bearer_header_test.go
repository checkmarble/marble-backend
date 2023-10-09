package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

func TestParseAuthorizationBearerHeader_Norminal(t *testing.T) {
	header := http.Header{}
	header.Add("Authorization", "Bearer TOKEN")

	authorization, err := ParseAuthorizationBearerHeader(header)
	assert.NoError(t, err)
	assert.Equal(t, authorization, "TOKEN")
}

func TestParseAuthorizationBearerHeader_EmptyHeader(t *testing.T) {

	authorization, err := ParseAuthorizationBearerHeader(http.Header{})
	assert.NoError(t, err)
	assert.Empty(t, authorization)
}

func TestParseAuthorizationBearerHeader_BadBearerFormat(t *testing.T) {
	header := http.Header{}
	header.Add("Authorization", "MalformedBearer")

	_, err := ParseAuthorizationBearerHeader(header)
	assert.ErrorIs(t, err, models.UnAuthorizedError)
}
