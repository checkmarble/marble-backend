package dto

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestCredentialsDtoRoleBindingBundle(t *testing.T) {
	credentials := models.Credentials{
		ActorIdentity: models.Identity{UserId: "user_id"},
		RoleBindingBundle: models.RoleBindingBundle{
			UsedSecondFactor: true,
		},
	}

	credentialsDto, err := AdaptCredentialDto(credentials)
	assert.NoError(t, err)

	encoded, err := json.Marshal(credentialsDto)
	assert.NoError(t, err)

	var decodedDto Credentials
	assert.NoError(t, json.Unmarshal(encoded, &decodedDto))

	decoded := AdaptCredential(decodedDto)
	assert.True(t, decoded.RoleBindingBundle.UsedSecondFactor, "the second factor must survive a token round-trip")
}
