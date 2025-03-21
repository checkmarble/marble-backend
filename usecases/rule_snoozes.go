package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type decisionGetter interface {
	DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error)
}
type iterationGetter interface {
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)
}

type ruleSnoozeRepository interface {
	GetSnoozeById(ctx context.Context, exec repositories.Executor, ruleSnoozeId string) (models.RuleSnooze, error)
	CreateSnoozeGroup(ctx context.Context, exec repositories.Executor, id, organizationId string) error
	ListActiveRuleSnoozesForDecision(
		ctx context.Context,
		exec repositories.Executor,
		snoozeGroupIds []string,
		pivotValue string,
	) ([]models.RuleSnooze, error)
	AnySnoozesForIteration(
		ctx context.Context,
		exec repositories.Executor,
		snoozeGroupIds []string,
	) (map[string]bool, error)
	CreateRuleSnooze(
		ctx context.Context,
		exec repositories.Executor,
		input models.RuleSnoozeCreateInput,
	) error
}

type enforceSecuritySnoozes interface {
	ReadSnoozesOfDecision(ctx context.Context, decision models.Decision) error
	CreateSnoozesOnDecision(ctx context.Context, decision models.Decision) error
	ReadSnoozesOfIteration(ctx context.Context, iteration models.ScenarioIteration) error
	ReadRuleSnooze(ctx context.Context, snooze models.RuleSnooze) error
}

type updateRuleRepository interface {
	UpdateRule(ctx context.Context, exec repositories.Executor, rule models.UpdateRuleInput) error
}

type caseUsecase interface {
	GetCase(ctx context.Context, caseId string) (models.Case, error)
	CreateRuleSnoozeEvent(ctx context.Context, tx repositories.Transaction, input models.RuleSnoozeCaseEventInput,
	) error
}

type webhooksSender interface {
	SendWebhookEventAsync(ctx context.Context, webhookEventId string)
}

type RuleSnoozeUsecase struct {
	decisionGetter       decisionGetter
	executorFactory      executor_factory.ExecutorFactory
	transactionFactory   executor_factory.TransactionFactory
	caseUsecase          caseUsecase
	iterationGetter      iterationGetter
	ruleRepository       updateRuleRepository
	ruleSnoozeRepository ruleSnoozeRepository
	enforceSecurity      enforceSecuritySnoozes
	webhooksSender       webhooksSender
}

func NewRuleSnoozeUsecase(
	d decisionGetter,
	e executor_factory.ExecutorFactory,
	t executor_factory.TransactionFactory,
	cr caseUsecase,
	ig iterationGetter,
	r updateRuleRepository,
	s ruleSnoozeRepository,
	es enforceSecuritySnoozes,
	w webhooksSender,
) RuleSnoozeUsecase {
	return RuleSnoozeUsecase{
		decisionGetter:       d,
		executorFactory:      e,
		transactionFactory:   t,
		caseUsecase:          cr,
		iterationGetter:      ig,
		ruleRepository:       r,
		ruleSnoozeRepository: s,
		enforceSecurity:      es,
		webhooksSender:       w,
	}
}

func (usecase RuleSnoozeUsecase) ActiveSnoozesForDecision(ctx context.Context, decisionId string) (models.SnoozesOfDecision, error) {
	exec := usecase.executorFactory.NewExecutor()
	decisions, err := usecase.decisionGetter.DecisionsById(ctx, exec, []string{decisionId})
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}
	if len(decisions) == 0 {
		return models.SnoozesOfDecision{}, errors.Wrapf(models.NotFoundError, "decision %s not found", decisionId)
	}
	decision := decisions[0]

	if err := usecase.enforceSecurity.ReadSnoozesOfDecision(ctx, decision); err != nil {
		return models.SnoozesOfDecision{}, err
	}

	it, err := usecase.iterationGetter.GetScenarioIteration(ctx, exec, decision.ScenarioIterationId)
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}

	if decision.PivotValue == nil {
		// no snoozes possible if decision doesn't have a pivot value
		return models.SnoozesOfDecision{
			DecisionId:  decisionId,
			RuleSnoozes: make([]models.RuleSnoozeWithRuleId, 0),
			Iteration:   it,
		}, nil
	}

	snoozeGroupIds := make([]string, 0, len(it.Rules))
	for _, rule := range it.Rules {
		if rule.SnoozeGroupId != nil {
			snoozeGroupIds = append(snoozeGroupIds, *rule.SnoozeGroupId)
		}
	}

	snoozes, err := usecase.ruleSnoozeRepository.ListActiveRuleSnoozesForDecision(
		ctx, exec, snoozeGroupIds, *decision.PivotValue)
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}

	return models.NewSnoozesOfDecision(decisionId, snoozes, it), nil
}

func (usecase RuleSnoozeUsecase) SnoozeDecision(
	ctx context.Context, input models.SnoozeDecisionInput,
) (models.SnoozesOfDecision, error) {
	exec := usecase.executorFactory.NewExecutor()

	duration, err := time.ParseDuration(input.Duration)
	if err != nil {
		return models.SnoozesOfDecision{}, errors.WithDetail(models.BadParameterError, err.Error())
	}
	if duration < 0 || duration > 180*24*time.Hour {
		return models.SnoozesOfDecision{}, errors.WithDetail(
			models.BadParameterError,
			"duration must be positive and below 180 days")
	}

	decisions, err := usecase.decisionGetter.DecisionsById(ctx, exec, []string{input.DecisionId})
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}
	if len(decisions) == 0 {
		return models.SnoozesOfDecision{}, errors.WithDetail(
			models.NotFoundError,
			fmt.Sprintf("decision %s not found", input.DecisionId))
	}
	decision := decisions[0]

	if decision.Case == nil {
		return models.SnoozesOfDecision{}, errors.WithDetail(
			models.UnprocessableEntityError,
			fmt.Sprintf("decision %s is not attached to a case and cannot be snoozed", input.DecisionId))
	}
	// case (inbox) permission check done in caseUsecase
	_, err = usecase.caseUsecase.GetCase(ctx, decision.Case.Id)
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}

	if decision.PivotValue == nil || *decision.PivotValue == "" {
		return models.SnoozesOfDecision{}, errors.WithDetail(
			models.UnprocessableEntityError,
			fmt.Sprintf("Decision %s has no pivot value and cannot be snoozed", decision.DecisionId))
	}

	if err := usecase.enforceSecurity.CreateSnoozesOnDecision(ctx, decision); err != nil {
		return models.SnoozesOfDecision{}, err
	}

	it, err := usecase.iterationGetter.GetScenarioIteration(ctx, exec, decision.ScenarioIterationId)
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}

	// verify input rule is in the decision
	ruleFound := false
	ruleIdx := 0
	thisRule := models.Rule{}
	snoozeGroupIds := make([]string, 0, len(it.Rules))
	for i, rule := range it.Rules {
		if rule.Id == input.RuleId {
			ruleFound = true
			thisRule = rule
			ruleIdx = i
		}
		if rule.SnoozeGroupId != nil {
			snoozeGroupIds = append(snoozeGroupIds, *rule.SnoozeGroupId)
		}
	}

	if !ruleFound {
		return models.SnoozesOfDecision{}, errors.WithDetail(
			models.NotFoundError,
			fmt.Sprintf("rule %s not found in decision %s", input.RuleId, input.DecisionId))
	}
	snoozeGroupId := thisRule.SnoozeGroupId

	if snoozeGroupId != nil {
		snoozes, err := usecase.ruleSnoozeRepository.ListActiveRuleSnoozesForDecision(ctx, exec, []string{
			*snoozeGroupId,
		}, *decision.PivotValue)
		if err != nil {
			return models.SnoozesOfDecision{}, err
		}

		if len(snoozes) > 0 {
			return models.SnoozesOfDecision{}, errors.WithDetail(
				models.ConflictError,
				fmt.Sprintf("rule %s already has an active snooze %s", input.RuleId, input.DecisionId))
		}
	}

	webhookEventId := uuid.NewString()
	snoozes, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) ([]models.RuleSnooze, error) {
			if snoozeGroupId == nil {
				val := uuid.NewString()
				snoozeGroupId = &val
				snoozeGroupIds = append(snoozeGroupIds, *snoozeGroupId)
				err := usecase.ruleSnoozeRepository.CreateSnoozeGroup(ctx, tx, val, input.OrganizationId)
				if err != nil {
					return nil, err
				}
				err = usecase.ruleRepository.UpdateRule(ctx, tx, models.UpdateRuleInput{
					Id:            thisRule.Id,
					SnoozeGroupId: snoozeGroupId,
				})
				if err != nil {
					return nil, err
				}
				// "it" variable is reused in NewSnoozesOfDecision() at the end of the function to return the created snooze
				// update it here rather than re-reading it from the DB
				it.Rules[ruleIdx].SnoozeGroupId = snoozeGroupId
			}
			snoozeId := uuid.NewString()
			err = usecase.ruleSnoozeRepository.CreateRuleSnooze(ctx, tx, models.RuleSnoozeCreateInput{
				Id:                    snoozeId,
				CreatedByUserId:       input.UserId,
				ExpiresAt:             time.Now().Add(duration),
				CreatedFromDecisionId: input.DecisionId,
				CreatedFromRuleId:     thisRule.Id,
				PivotValue:            *decision.PivotValue,
				SnoozeGroupId:         *snoozeGroupId,
			})
			if err != nil {
				return nil, err
			}

			var userId *string = nil

			if input.UserId != nil {
				userId = utils.Ptr(string(*input.UserId))
			}

			err = usecase.caseUsecase.CreateRuleSnoozeEvent(ctx, tx, models.RuleSnoozeCaseEventInput{
				CaseId:         decision.Case.Id,
				Comment:        input.Comment,
				RuleSnoozeId:   snoozeId,
				UserId:         userId,
				WebhookEventId: webhookEventId,
			})
			if err != nil {
				return nil, err
			}

			return usecase.ruleSnoozeRepository.ListActiveRuleSnoozesForDecision(ctx, tx, snoozeGroupIds, *decision.PivotValue)
		},
	)

	// TODO: do we really ignore errors here?

	usecase.webhooksSender.SendWebhookEventAsync(ctx, webhookEventId)

	return models.NewSnoozesOfDecision(decision.DecisionId, snoozes, it), nil
}

func (usecase RuleSnoozeUsecase) ActiveSnoozesForScenarioIteration(ctx context.Context, iterationId string) (models.SnoozesOfIteration, error) {
	exec := usecase.executorFactory.NewExecutor()
	it, err := usecase.iterationGetter.GetScenarioIteration(ctx, exec, iterationId)
	if err != nil {
		return models.SnoozesOfIteration{}, err
	}

	if err := usecase.enforceSecurity.ReadSnoozesOfIteration(ctx, it); err != nil {
		return models.SnoozesOfIteration{}, err
	}

	snoozeGroupIds := make([]string, 0, len(it.Rules))
	for _, rule := range it.Rules {
		if rule.SnoozeGroupId != nil {
			snoozeGroupIds = append(snoozeGroupIds, *rule.SnoozeGroupId)
		}
	}
	snoozesByRule, err := usecase.ruleSnoozeRepository.AnySnoozesForIteration(
		ctx, exec, snoozeGroupIds)
	if err != nil {
		return models.SnoozesOfIteration{}, err
	}

	snoozes := make([]models.RuleSnoozeInformation, 0, len(it.Rules))
	for _, rule := range it.Rules {
		if rule.SnoozeGroupId == nil {
			snoozes = append(snoozes, models.RuleSnoozeInformation{
				RuleId:           rule.Id,
				SnoozeGroupId:    "",
				HasSnoozesActive: false,
			})
			continue
		}
		hasSnoozesActive, ok := snoozesByRule[*rule.SnoozeGroupId]
		if !ok {
			hasSnoozesActive = false
		}
		snoozes = append(snoozes, models.RuleSnoozeInformation{
			RuleId:           rule.Id,
			SnoozeGroupId:    *rule.SnoozeGroupId,
			HasSnoozesActive: hasSnoozesActive,
		})
	}

	return models.SnoozesOfIteration{
		IterationId: iterationId,
		RuleSnoozes: snoozes,
	}, nil
}

func (usecase RuleSnoozeUsecase) GetRuleSnoozeById(ctx context.Context, ruleSnoozeId string) (models.RuleSnooze, error) {
	s, err := usecase.ruleSnoozeRepository.GetSnoozeById(
		ctx, usecase.executorFactory.NewExecutor(), ruleSnoozeId)
	if err != nil {
		return models.RuleSnooze{}, err
	}

	if err := usecase.enforceSecurity.ReadRuleSnooze(ctx, s); err != nil {
		return models.RuleSnooze{}, err
	}

	return s, nil
}
