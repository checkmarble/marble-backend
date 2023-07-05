package organization

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"marble/marble-backend/models"
	"marble/marble-backend/models/operators"
	"marble/marble-backend/repositories"
	"marble/marble-backend/utils"

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

type organizationSeederImpl struct {
	TransactionFactory repositories.TransactionFactory
	Repositories       repositories.Repositories
}

func NewOrganizationSeeder(repo repositories.Repositories, transactionFactory repositories.TransactionFactory) OrganizationSeeder {
	return &organizationSeederImpl{
		TransactionFactory: transactionFactory,
		Repositories:       repo,
	}
}

func (o *organizationSeederImpl) Seed(orgId string) error {

	///////////////////////////////
	// Tokens
	///////////////////////////////

	err := o.Repositories.ApiKeyRepository.CreateApiKey(nil, models.CreateApiKeyInput{
		OrganizationId: orgId,
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

	err = o.Repositories.CustomListRepository.CreateCustomList(nil, models.CreateCustomListInput{
		OrgId:       orgId,
		Name:        "Welcome to Marble",
		Description: "Need a whitelist or blacklist ? The list is your friend :)",
	}, newCustomListId)
	if err != nil {
		return err
	}

	addCustomListValueInput := models.AddCustomListValueInput{
		OrgId:        orgId,
		CustomListId: newCustomListId,
		Value: "Welcome",
	}
	o.Repositories.CustomListRepository.AddCustomListValue(nil, addCustomListValueInput, uuid.NewString())
	addCustomListValueInput.Value = "to"
	o.Repositories.CustomListRepository.AddCustomListValue(nil, addCustomListValueInput, uuid.NewString())
	addCustomListValueInput.Value = "marble"
	o.Repositories.CustomListRepository.AddCustomListValue(nil, addCustomListValueInput, uuid.NewString())

	///////////////////////////////
	// Create and store a scenario
	///////////////////////////////
	createScenarioInput := models.CreateScenarioInput{
		Name:              "test name",
		Description:       "test description",
		TriggerObjectType: "transactions",
	}
	scenario, err := o.Repositories.ScenarioWriteRepository.CreateScenario(context.TODO(), orgId, createScenarioInput)
	if err != nil {
		log.Printf("error creating scenario: %v", err)
		return err
	}

	createScenarioIterationInput := models.CreateScenarioIterationInput{
		ScenarioID: scenario.ID,
		Body: &models.CreateScenarioIterationBody{
			TriggerCondition:     &operators.BoolValue{Value: true},
			ScoreReviewThreshold: utils.Ptr(10),
			ScoreRejectThreshold: utils.Ptr(30),
			Rules: []models.CreateRuleInput{
				{
					Formula:       &operators.BoolValue{Value: true},
					ScoreModifier: 2,
					Name:          "Rule 1 Name",
					Description:   "Rule 1 Desc",
				},
				{
					Formula:       &operators.BoolValue{Value: false},
					ScoreModifier: 2,
					Name:          "Rule 2 Name",
					Description:   "Rule 2 Desc",
				},
				{
					Formula:       &operators.EqBool{Left: &operators.BoolValue{Value: true}, Right: &operators.BoolValue{Value: true}},
					ScoreModifier: 2,
					Name:          "Rule 3 Name",
					Description:   "Rule 3 Desc",
				},
				{
					Formula:       &operators.EqBool{Left: &operators.BoolValue{Value: true}, Right: &operators.EqBool{Left: &operators.BoolValue{Value: false}, Right: &operators.BoolValue{Value: false}}},
					ScoreModifier: 2,
					Name:          "Rule 4 Name",
					Description:   "Rule 4 Desc",
				},
				{
					Formula:       &operators.StringIsInList{Str: &operators.StringValue{Value: "marble"}, List: &operators.DbCustomListStringArray{CustomListId: newCustomListId}},
					ScoreModifier: 2,
					Name:          "Rule 5 Name",
					Description:   "Rule 5 Desc",
				},
			},
		},
	}

	scenarioIteration, err := o.Repositories.ScenarioIterationWriteRepository.CreateScenarioIteration(context.TODO(), orgId, createScenarioIterationInput)
	if err != nil {
		log.Printf("error creating scenario iteration: %v", err)
		return err
	}
	_, err = o.Repositories.ScenarioPublicationRepository.CreateScenarioPublication(context.TODO(), orgId, models.CreateScenarioPublicationInput{
		ScenarioIterationID: scenarioIteration.ID,
		PublicationAction:   models.Publish,
	})
	if err != nil {
		log.Printf("error publishing scenario iteration: %v", err)
		return err
	}

	///////////////////////////////
	// Also create the demo scenario
	///////////////////////////////
	demoScenario, err := o.Repositories.ScenarioWriteRepository.CreateScenario(context.TODO(), orgId, models.CreateScenarioInput{
		Name:              "Demo scenario",
		Description:       "Demo scenario",
		TriggerObjectType: "transactions",
	})
	if err != nil {
		log.Printf("error creating demo scenario: %v", err)
		return err
	}

	createDemoScenarioIterationInput := models.CreateScenarioIterationInput{
		ScenarioID: demoScenario.ID,
		Body: &models.CreateScenarioIterationBody{
			TriggerCondition: &operators.And{
				Operands: []operators.OperatorBool{
					&operators.EqString{
						Left:  &operators.PayloadFieldString{FieldName: "direction"},
						Right: &operators.StringValue{Value: "payout"},
					},
					&operators.EqString{
						Left:  &operators.PayloadFieldString{FieldName: "status"},
						Right: &operators.StringValue{Value: "pending"},
					},
				},
			},
			ScoreReviewThreshold: utils.Ptr(20),
			ScoreRejectThreshold: utils.Ptr(30),
			Schedule:             "*/10 * * * *",
			Rules: []models.CreateRuleInput{
				{
					Formula: &operators.And{
						Operands: []operators.OperatorBool{
							&operators.GreaterOrEqualFloat{
								Left:  &operators.PayloadFieldFloat{FieldName: "amount"},
								Right: &operators.FloatValue{Value: 10000},
							},
							&operators.LesserFloat{
								Left:  &operators.PayloadFieldFloat{FieldName: "amount"},
								Right: &operators.FloatValue{Value: 100000},
							},
						},
					},
					ScoreModifier: 10,
					Name:          "Medium amount",
					Description:   "Amount is between 10k and 100k, hence medium risk",
				},
				{
					Formula: &operators.GreaterOrEqualFloat{
						Left:  &operators.PayloadFieldFloat{FieldName: "amount"},
						Right: &operators.FloatValue{Value: 100000},
					},
					ScoreModifier: 20,
					Name:          "High amount",
					Description:   "Amount is greater than 100k, hence high risk",
				},
				{
					Formula: &operators.StringIsInList{
						Str: &operators.PayloadFieldString{FieldName: "bic_country"},
						List: &operators.StringListValue{
							Value: []string{"HU", "IT", "PO", "IR"},
						},
					},
					ScoreModifier: 10,
					Name:          "Medium risk country",
					Description:   "Country is in the list of medium risk (european) countries",
				},
				{
					Formula: &operators.StringIsInList{
						Str: &operators.PayloadFieldString{FieldName: "bic_country"},
						List: &operators.StringListValue{
							Value: []string{"RO", "RU", "LT"},
						},
					},
					ScoreModifier: 20,
					Name:          "High risk country",
					Description:   "Country is in the list of high risk (european) countries",
				},
				{
					Formula: &operators.EqString{
						Left: &operators.PayloadFieldString{FieldName: "bic_country"},
						Right: &operators.StringValue{
							Value: "FR",
						},
					},
					ScoreModifier: -10,
					Name:          "Low risk country",
					Description:   "Country is domestic (France)",
				},
				{
					Formula: &operators.StringIsInList{
						Str: &operators.PayloadFieldString{FieldName: "bic_country"},
						List: &operators.StringListValue{
							Value: []string{"FRTRZOFRPP", "FPPRPFFXXX"},
						},
					},
					ScoreModifier: 10,
					Name:          "High risk BIC",
					Description:   "BIC is in the list of known high risk BICs",
				},
				{
					Formula: &operators.EqBool{
						Left: &operators.DbFieldBool{
							FieldName:        "is_frozen",
							TriggerTableName: "transactions",
							Path:             []string{"account"},
						},
						Right: &operators.BoolValue{
							Value: true,
						},
					},
					ScoreModifier: 100,
					Name:          "Frozen account",
					Description:   "The account is frozen",
				},
				{
					Formula: &operators.EqString{
						Left: &operators.DbFieldString{
							FieldName:        "name",
							TriggerTableName: "transactions",
							Path:             []string{"account", "company"},
						},
						Right: &operators.StringValue{Value: "Company 1"},
					},
					ScoreModifier: 1,
					Name:          "Test auto-fail rule",
					Description:   "This rule fails for testing purposes, if the owner company has not been ingested",
				},
			},
		},
	}
	demoScenarioIteration, err := o.Repositories.ScenarioIterationWriteRepository.CreateScenarioIteration(context.TODO(), orgId, createDemoScenarioIterationInput)
	if err != nil {
		log.Printf("error creating demo scenario iteration: %v", err)
		return err
	}
	_, err = o.Repositories.ScenarioPublicationRepository.CreateScenarioPublication(context.TODO(), orgId, models.CreateScenarioPublicationInput{
		ScenarioIterationID: demoScenarioIteration.ID,
		PublicationAction:   models.Publish,
	})
	if err != nil {
		log.Printf("error publishing demo scenario iteration: %v", err)
		return err
	}

	log.Println("")
	log.Println("Finish to Seed the DB")
	return nil
}
