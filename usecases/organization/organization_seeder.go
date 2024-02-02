package organization

import (
	"context"
	"log"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"

	"github.com/google/uuid"
)

type OrganizationSeeder struct {
	CustomListRepository repositories.CustomListRepository
}

func (o *OrganizationSeeder) Seed(ctx context.Context, organizationId string) error {
	///////////////////////////////
	// Create and store a custom list
	///////////////////////////////
	newCustomListId := uuid.NewString()

	err := o.CustomListRepository.CreateCustomList(ctx, nil, models.CreateCustomListInput{
		Name:        "Welcome to Marble",
		Description: "Need a whitelist or blacklist ? The list is your friend :)",
	}, organizationId, newCustomListId)
	if err != nil {
		return err
	}

	addCustomListValueInput := models.AddCustomListValueInput{
		CustomListId: newCustomListId,
		Value:        "Welcome",
	}
	o.CustomListRepository.AddCustomListValue(ctx, nil, addCustomListValueInput, uuid.NewString())
	addCustomListValueInput.Value = "to"
	o.CustomListRepository.AddCustomListValue(ctx, nil, addCustomListValueInput, uuid.NewString())
	addCustomListValueInput.Value = "marble"
	o.CustomListRepository.AddCustomListValue(ctx, nil, addCustomListValueInput, uuid.NewString())

	log.Println("")
	log.Println("Finish to Seed the DB")
	return nil
}
