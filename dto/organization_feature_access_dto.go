package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type APIOrganizationFeatureAccess struct {
	TestRun         string `json:"test_run"`
	Workflows       string `json:"workflows"`
	Webhooks        string `json:"webhooks"`
	RuleSnoozes     string `json:"rule_snoozes"`
	Roles           string `json:"roles"`
	Analytics       string `json:"analytics"`
	Sanctions       string `json:"sanctions"`
	NameRecognition string `json:"name_recognition"`
	CaseAutoAssign  string `json:"case_auto_assign"`
	CaseAiAssist    string `json:"case_ai_assist"`

	// user-scoped
	// Currently only used to control display of the AI assist button in the UI - DO NOT use for anything else as it will be removed
	AiAssist string `json:"ai_assist"`
}

func AdaptOrganizationFeatureAccessDto(f models.OrganizationFeatureAccess) APIOrganizationFeatureAccess {
	return APIOrganizationFeatureAccess{
		TestRun:         f.TestRun.String(),
		Workflows:       f.Workflows.String(),
		Webhooks:        f.Webhooks.String(),
		RuleSnoozes:     f.RuleSnoozes.String(),
		Roles:           f.Roles.String(),
		Analytics:       f.Analytics.String(),
		Sanctions:       f.Sanctions.String(),
		NameRecognition: f.NameRecognition.String(),
		CaseAutoAssign:  f.CaseAutoAssign.String(),
		CaseAiAssist:    f.CaseAiAssist.String(),
		AiAssist:        f.AiAssist.String(),
	}
}

type UpdateOrganizationFeatureAccessBodyDto struct {
	TestRun   *string `json:"test_run"`
	Sanctions *string `json:"sanctions"`
}

func AdaptUpdateOrganizationFeatureAccessInput(f UpdateOrganizationFeatureAccessBodyDto,
	orgId uuid.UUID,
) models.UpdateOrganizationFeatureAccessInput {
	var testRun, sanctions *models.FeatureAccess
	if f.TestRun != nil {
		testRun = utils.Ptr(models.FeatureAccessFrom(*f.TestRun))
	}
	if f.Sanctions != nil {
		sanctions = utils.Ptr(models.FeatureAccessFrom(*f.Sanctions))
	}
	return models.UpdateOrganizationFeatureAccessInput{
		OrganizationId: orgId,
		TestRun:        testRun,
		Sanctions:      sanctions,
	}
}
