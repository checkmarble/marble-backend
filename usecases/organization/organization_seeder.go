package organization

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/scenarios"

	"github.com/google/uuid"
)

func randomAPiKey() string {
	var key = make([]byte, 8)
	_, err := rand.Read(key)
	if err != nil {
		panic(fmt.Errorf("randomAPiKey: %w", err))
	}
	return hex.EncodeToString(key)
}

type OrganizationSeeder interface {
	Seed(organizationId string) error
}

type OrganizationSeederImpl struct {
	CustomListRepository             repositories.CustomListRepository
	ApiKeyRepository                 repositories.ApiKeyRepository
	ScenarioWriteRepository          repositories.ScenarioWriteRepository
	ScenarioIterationWriteRepository repositories.ScenarioIterationWriteRepository
	ScenarioPublisher                scenarios.ScenarioPublisher
}

func (o *OrganizationSeederImpl) Seed(organizationId string) error {

	///////////////////////////////
	// Tokens
	///////////////////////////////

	err := o.ApiKeyRepository.CreateApiKey(nil, models.CreateApiKeyInput{
		OrganizationId: organizationId,
		Key:            randomAPiKey(),
	})
	if err != nil {
		log.Printf("error creating token: %v", err)
		return err
	}

	///////////////////////////////
	// Create and store a custom list
	///////////////////////////////
	newCustomListId := uuid.NewString()

	err = o.CustomListRepository.CreateCustomList(nil, models.CreateCustomListInput{
		OrgId:       organizationId,
		Name:        "Welcome to Marble",
		Description: "Need a whitelist or blacklist ? The list is your friend :)",
	}, newCustomListId)
	if err != nil {
		return err
	}

	addCustomListValueInput := models.AddCustomListValueInput{
		OrgId:        organizationId,
		CustomListId: newCustomListId,
		Value:        "Welcome",
	}
	o.CustomListRepository.AddCustomListValue(nil, addCustomListValueInput, uuid.NewString())
	addCustomListValueInput.Value = "to"
	o.CustomListRepository.AddCustomListValue(nil, addCustomListValueInput, uuid.NewString())
	addCustomListValueInput.Value = "marble"
	o.CustomListRepository.AddCustomListValue(nil, addCustomListValueInput, uuid.NewString())

	log.Println("")
	log.Println("Finish to Seed the DB")
	return nil
}
