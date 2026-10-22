package dto

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestCredentialsDtoRoleBindingBundle(t *testing.T) {
	credentials := models.Credentials{
		ActorIdentity: models.Identity{UserId: "user_id"},
		RoleBindingBundle: models.RoleBindingBundle{
			UsedSecondFactor: true,
			ClientIp:         net.ParseIP("10.1.2.3"),
		},
	}

	credentialsDto, err := AdaptCredentialDto(credentials)
	assert.NoError(t, err)

	encoded, err := json.Marshal(credentialsDto)
	assert.NoError(t, err)
	assert.NotContains(t, string(encoded), "10.1.2.3", "the client IP must never be written in a token")

	var decodedDto Credentials
	assert.NoError(t, json.Unmarshal(encoded, &decodedDto))

	decoded := AdaptCredential(decodedDto)
	assert.True(t, decoded.RoleBindingBundle.UsedSecondFactor, "the second factor must survive a token round-trip")
	assert.Nil(t, decoded.RoleBindingBundle.ClientIp)
}
