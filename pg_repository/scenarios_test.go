package pg_repository

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/models/operators"
	"testing"

	sq "github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
)

func cleanupScenarios(ids []string, orgID string) error {
	sql, args, err := globalTestParams.repository.queryBuilder.
		Delete("").
		From("scenarios").
		Where(sq.Eq{"id": ids}).
		Where("org_id = ?", globalTestParams.testIds["OrganizationId"]).
		ToSql()
	if err != nil {
		return err
	}
	_, err = globalTestParams.repository.db.Exec(context.Background(), sql, args...)
	return err
}

func TestCreateScenario(t *testing.T) {
	t.SkipNow()
	scenar, err := globalTestParams.repository.CreateScenario(
		context.Background(),
		globalTestParams.testIds["OrganizationId"],
		models.CreateScenarioInput{
			Name:              "Scenario 1",
			Description:       "This is a test scenario",
			TriggerObjectType: "transactions",
		})
	if err != nil {
		t.Fatalf("Could not create scenario: %s", err)
	}

	asserts := assert.New(t)
	asserts.Equal("Scenario 1", scenar.Name)
	asserts.Equal("This is a test scenario", scenar.Description)
	asserts.Equal("transactions", scenar.TriggerObjectType)
	asserts.Regexp(uuidRegexp, scenar.ID)
	asserts.True(scenar.LiveVersionID == nil)

	cleanupScenarios([]string{scenar.ID}, globalTestParams.testIds["OrganizationId"])
	if err != nil {
		t.Fatalf("Could not cleanup scenarios: %s", err)
	}
}

func TestUpdateScenario(t *testing.T) {
	t.SkipNow()
	scenar, err := globalTestParams.repository.CreateScenario(
		context.Background(),
		globalTestParams.testIds["OrganizationId"],
		models.CreateScenarioInput{
			Name:              "Scenario 1",
			Description:       "This is a test scenario",
			TriggerObjectType: "transactions",
		})
	if err != nil {
		t.Fatalf("Could not create scenario: %s", err)
	}

	var (
		newName = "New name"
		newDesc = "New description"
	)
	newScenar, err := globalTestParams.repository.UpdateScenario(
		context.Background(),
		globalTestParams.testIds["OrganizationId"],
		models.UpdateScenarioInput{
			ID:          scenar.ID,
			Name:        &newName,
			Description: &newDesc,
		},
	)
	if err != nil {
		t.Fatalf("Could not update scenario: %s", err)
	}

	asserts := assert.New(t)
	asserts.Equal("New name", newScenar.Name)
	asserts.Equal("New description", newScenar.Description)
	asserts.Equal("transactions", newScenar.TriggerObjectType)
	asserts.Equal(scenar.ID, newScenar.ID)
	asserts.True(newScenar.LiveVersionID == nil)

	cleanupScenarios([]string{scenar.ID}, globalTestParams.testIds["OrganizationId"])
	if err != nil {
		t.Fatalf("Could not cleanup scenarios: %s", err)
	}

}

func TestListScenarios(t *testing.T) {
	t.SkipNow()
	scenar1, err := globalTestParams.repository.CreateScenario(
		context.Background(),
		globalTestParams.testIds["OrganizationId"],
		models.CreateScenarioInput{
			Name:              "Scenario 1",
			Description:       "This is a test scenario",
			TriggerObjectType: "transactions",
		})
	if err != nil {
		t.Fatalf("Could not create scenario: %s", err)
	}
	scenar2, err := globalTestParams.repository.CreateScenario(
		context.Background(),
		globalTestParams.testIds["OrganizationId"],
		models.CreateScenarioInput{
			Name:              "Scenario 2",
			Description:       "This is another test scenario",
			TriggerObjectType: "transactions",
		})
	if err != nil {
		t.Fatalf("Could not create scenario: %s", err)
	}

	scenarios, err := globalTestParams.repository.ListScenarios(
		context.Background(),
		globalTestParams.testIds["OrganizationId"],
	)

	if err != nil {
		t.Fatalf("Could not list scenarios: %s", err)
	}

	asserts := assert.New(t)
	asserts.Equal([]models.Scenario{scenar1, scenar2}, scenarios)
	asserts.Equal(2, len(scenarios))

	cleanupScenarios([]string{scenar1.ID, scenar2.ID}, globalTestParams.testIds["OrganizationId"])
	if err != nil {
		t.Fatalf("Could not cleanup scenarios: %s", err)
	}
}

func TestGetScenarioWithLiveVersion(t *testing.T) {
	t.SkipNow()
	scenar, err := globalTestParams.repository.CreateScenario(
		context.Background(),
		globalTestParams.testIds["OrganizationId"],
		models.CreateScenarioInput{
			Name:              "Scenario 1",
			Description:       "This is a test scenario",
			TriggerObjectType: "transactions",
		})
	if err != nil {
		t.Fatalf("Could not create scenario: %s", err)
	}

	score := 10
	iteration, err := globalTestParams.repository.CreateScenarioIteration(context.Background(), globalTestParams.testIds["OrganizationId"], models.CreateScenarioIterationInput{
		ScenarioID: scenar.ID,
		Body: &models.CreateScenarioIterationBody{
			TriggerCondition: &operators.BoolValue{Value: true},
			Rules: []models.CreateRuleInput{
				{
					Formula:       &operators.BoolValue{Value: true},
					ScoreModifier: 2,
					Name:          "Rule 1 Name",
					Description:   "Rule 1 Desc",
				},
			},
			ScoreReviewThreshold: &score,
			ScoreRejectThreshold: &score,
		},
	})
	if err != nil {
		t.Fatalf("Could not create scenario iteration: %s", err)
	}

	_, err = globalTestParams.repository.CreateScenarioPublication(context.Background(), globalTestParams.testIds["OrganizationId"], models.CreateScenarioPublicationInput{
		ScenarioIterationID: iteration.ID,
		PublicationAction:   models.PublicationActionFrom("publish"),
	})
	if err != nil {
		t.Fatalf("Could not create scenario publication: %s", err)
	}

	scenarWithLiveVersion, err := globalTestParams.repository.GetScenario(
		context.Background(),
		globalTestParams.testIds["OrganizationId"],
		scenar.ID,
	)
	if err != nil {
		t.Fatalf("Could not get scenario: %s", err)
	}

	asserts := assert.New(t)
	asserts.Equal(scenar.ID, scenarWithLiveVersion.ID)
	asserts.Equal(scenar.Name, scenarWithLiveVersion.Name)
	asserts.Equal(scenar.Description, scenarWithLiveVersion.Description)
	asserts.Equal(scenar.TriggerObjectType, scenarWithLiveVersion.TriggerObjectType)
	asserts.Equal(iteration.ID, *scenarWithLiveVersion.LiveVersionID)

	cleanupScenarios([]string{scenar.ID}, globalTestParams.testIds["OrganizationId"])
	if err != nil {
		t.Fatalf("Could not cleanup scenarios: %s", err)
	}
}
