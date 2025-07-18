package metrics_collection

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

// Utility function to get the list of org ids and the map of org id to public id
// Use it in collector implementation to avoid duplicate code
func getOrgIDlistAndPublicIdMap(orgs []models.Organization) ([]string, map[string]uuid.UUID) {
	orgIds := make([]string, len(orgs))
	orgMap := make(map[string]uuid.UUID)
	for i, org := range orgs {
		orgIds[i] = org.Id
		orgMap[org.Id] = org.PublicId
	}

	return orgIds, orgMap
}
