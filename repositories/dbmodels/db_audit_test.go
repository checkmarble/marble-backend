package dbmodels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdaptActorlessAuditEvent(t *testing.T) {
	event, err := AdaptAuditEventWithActor(DbAuditEventWithActor{})
	require.NoError(t, err)
	require.Equal(t, "system", event.Actor.Type)
	require.Equal(t, "System", event.Actor.Name)
}
