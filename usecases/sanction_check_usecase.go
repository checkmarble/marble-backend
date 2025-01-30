package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

type SanctionCheckEnforceSecurityDecision interface {
	ReadDecision(models.Decision) error
}

type SanctionCheckEnforceSecurityCase interface {
	ReadOrUpdateCase(models.Case, []string) error
}

type SanctionCheckProvider interface {
	GetLatestLocalDataset(context.Context) (models.OpenSanctionsDataset, error)
	Search(context.Context, models.SanctionCheckConfig,
		models.OpenSanctionsQuery) (models.SanctionCheck, error)
}

type SanctionCheckDecisionRepository interface {
	DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error)
}

type SanctionCheckOrganizationRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
}

type SanctionCheckInboxReader interface {
	ListInboxes(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		withCaseCount bool,
	) ([]models.Inbox, error)
}

type SanctionCheckRepository interface {
	GetActiveSanctionCheckForDecision(context.Context, repositories.Executor, string) (models.SanctionCheck, error)
	GetSanctionCheck(context.Context, repositories.Executor, string) (models.SanctionCheck, error)
	ArchiveSanctionCheck(context.Context, repositories.Executor, string) error
	InsertSanctionCheck(context.Context, repositories.Executor, string,
		models.SanctionCheck) (models.SanctionCheck, error)
	UpdateSanctionCheckStatus(ctx context.Context, exec repositories.Executor, id string,
		status models.SanctionCheckStatus) (models.SanctionCheck, error)

	ListSanctionCheckMatches(ctx context.Context, exec repositories.Executor, sanctionCheckId string, forUpdate ...bool) (
		[]models.SanctionCheckMatch, error)
	GetSanctionCheckMatch(ctx context.Context, exec repositories.Executor, matchId string) (models.SanctionCheckMatch, error)
	UpdateSanctionCheckMatchStatus(ctx context.Context, exec repositories.Executor,
		match models.SanctionCheckMatch, update models.SanctionCheckMatchUpdate) (models.SanctionCheckMatch, error)
	AddSanctionCheckMatchComment(ctx context.Context, exec repositories.Executor,
		comment models.SanctionCheckMatchComment) (models.SanctionCheckMatchComment, error)
	ListSanctionCheckMatchComments(ctx context.Context, exec repositories.Executor, matchId string) (
		[]models.SanctionCheckMatchComment, error)
}

type SanctionCheckUsecase struct {
	enforceSecurityDecision SanctionCheckEnforceSecurityDecision
	enforceSecurityCase     SanctionCheckEnforceSecurityCase

	organizationRepository        SanctionCheckOrganizationRepository
	decisionRepository            SanctionCheckDecisionRepository
	inboxReader                   SanctionCheckInboxReader
	openSanctionsProvider         SanctionCheckProvider
	sanctionCheckConfigRepository SanctionCheckConfigRepository
	repository                    SanctionCheckRepository
	executorFactory               executor_factory.ExecutorFactory
	transactionFactory            executor_factory.TransactionFactory
}

func (uc SanctionCheckUsecase) CheckDataset(ctx context.Context) (models.OpenSanctionsDataset, error) {
	return uc.openSanctionsProvider.GetLatestLocalDataset(ctx)
}

func (uc SanctionCheckUsecase) GetSanctionCheck(ctx context.Context, decisionId string) (models.SanctionCheck, error) {
	decision, err := uc.decisionRepository.DecisionsById(ctx,
		uc.executorFactory.NewExecutor(), []string{decisionId})
	if err != nil {
		return models.SanctionCheck{}, err
	}
	if len(decision) == 0 {
		return models.SanctionCheck{}, errors.Wrap(models.NotFoundError, "requested decision does not exist")
	}

	if err := uc.enforceSecurityDecision.ReadDecision(decision[0]); err != nil {
		return models.SanctionCheck{}, err
	}

	sanctionCheck, err := uc.repository.GetActiveSanctionCheckForDecision(ctx,
		uc.executorFactory.NewExecutor(), decision[0].DecisionId)
	if err != nil {
		return models.SanctionCheck{}, errors.Wrap(err, "could not retrieve sanction check")
	}

	if sanctionCheck.Matches, err = uc.repository.ListSanctionCheckMatches(ctx,
		uc.executorFactory.NewExecutor(), sanctionCheck.Id); err != nil {
		return models.SanctionCheck{}, errors.Wrap(err,
			"could not retrieve sanction check matches")
	}

	sanctionCheck.Count = len(sanctionCheck.Matches)

	return sanctionCheck, nil
}

func (uc SanctionCheckUsecase) Execute(ctx context.Context, orgId string, cfg models.SanctionCheckConfig,
	query models.OpenSanctionsQuery,
) (models.SanctionCheck, error) {
	org, err := uc.organizationRepository.GetOrganizationById(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return models.SanctionCheck{},
			errors.Wrap(err, "could not retrieve organization")
	}

	query.OrgConfig = org.OpenSanctionsConfig

	matches, err := uc.openSanctionsProvider.Search(ctx, cfg, query)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	matches.Datasets = cfg.Datasets
	matches.OrgConfig = org.OpenSanctionsConfig

	return matches, err
}

func (uc SanctionCheckUsecase) Refine(ctx context.Context, refine models.SanctionCheckRefineRequest, requestedBy models.UserId) (models.SanctionCheck, error) {
	decision, sc, err := uc.enforceCanRefineSanctionCheck(ctx, refine.DecisionId)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	cfg, err := uc.sanctionCheckConfigRepository.GetSanctionCheckConfig(ctx,
		uc.executorFactory.NewExecutor(), decision.ScenarioIterationId)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	query := models.OpenSanctionsQuery{
		OrgConfig: sc.OrgConfig,
		Config:    cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"macron"},
		},
	}

	sanctionCheck, err := uc.Execute(ctx, decision.OrganizationId, cfg, query)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	sanctionCheck.IsManual = true
	sanctionCheck.RequestedBy = utils.Ptr(string(requestedBy))

	sanctionCheck, err = executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (models.SanctionCheck, error) {
		if err := uc.repository.ArchiveSanctionCheck(ctx, tx, decision.DecisionId); err != nil {
			return models.SanctionCheck{}, err
		}

		if sanctionCheck, err = uc.repository.InsertSanctionCheck(ctx, tx,
			decision.DecisionId, sanctionCheck); err != nil {
			return models.SanctionCheck{}, err
		}

		return sanctionCheck, err
	})
	if err != nil {
		return models.SanctionCheck{}, err
	}

	return sanctionCheck, nil
}

func (uc SanctionCheckUsecase) UpdateMatchStatus(
	ctx context.Context,
	update models.SanctionCheckMatchUpdate,
) (models.SanctionCheckMatch, error) {
	sanctionCheck, match, err := uc.enforceCanReadOrUpdateSanctionCheck(ctx, update.MatchId)
	if err != nil {
		return models.SanctionCheckMatch{}, err
	}

	if update.Status != models.SanctionMatchStatusConfirmedHit &&
		update.Status != models.SanctionMatchStatusNoHit {
		return match, errors.Wrap(models.BadParameterError,
			"invalid status received for sanction check match, should be 'confirmed_hit' or 'no_hit'")
	}

	if !sanctionCheck.IsReviewable() {
		return match, errors.Wrap(models.BadParameterError, "this sanction is not pending review")
	}

	if match.Status != models.SanctionMatchStatusPending {
		return match, errors.Wrap(models.BadParameterError, "this match is not pending review")
	}

	var updatedMatch models.SanctionCheckMatch
	err = uc.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			allMatches, err := uc.repository.ListSanctionCheckMatches(ctx, tx, sanctionCheck.Id, true)
			if err != nil {
				return err
			}
			pendingMatchesExcludingThis := utils.Filter(allMatches, func(m models.SanctionCheckMatch) bool {
				return m.Id != match.Id && m.Status == models.SanctionMatchStatusPending
			})

			updatedMatch, err = uc.repository.UpdateSanctionCheckMatchStatus(ctx, tx, match, update)
			if err != nil {
				return err
			}

			// If the match is confirmed, all other pending matches should be set to "skipped" and the sanction check to "confirmed_hit"
			if update.Status == models.SanctionMatchStatusConfirmedHit {
				for _, m := range pendingMatchesExcludingThis {
					_, err = uc.repository.UpdateSanctionCheckMatchStatus(ctx, tx, m, models.SanctionCheckMatchUpdate{
						MatchId:    m.Id,
						Status:     models.SanctionMatchStatusNoHit,
						ReviewerId: update.ReviewerId,
					})
					if err != nil {
						return err
					}
					// TODO: create a case event here
				}
			}

			// else, if it is the last match pending and it is not a hit, the sanction check should be set to "no_hit"
			if update.Status == models.SanctionMatchStatusNoHit && len(pendingMatchesExcludingThis) == 0 {
				_, err = uc.repository.UpdateSanctionCheckStatus(ctx, tx,
					sanctionCheck.Id, models.SanctionStatusNoHit)
				if err != nil {
					return err
				}
				// TODO: create a case event here
			}
			return nil
		},
	)

	return updatedMatch, err
}

func (uc SanctionCheckUsecase) MatchListComments(ctx context.Context, matchId string) ([]models.SanctionCheckMatchComment, error) {
	if _, _, err := uc.enforceCanReadOrUpdateSanctionCheck(ctx, matchId); err != nil {
		return nil, err
	}

	return uc.repository.ListSanctionCheckMatchComments(ctx, uc.executorFactory.NewExecutor(), matchId)
}

func (uc SanctionCheckUsecase) MatchAddComment(ctx context.Context, matchId string,
	comment models.SanctionCheckMatchComment,
) (models.SanctionCheckMatchComment, error) {
	if _, _, err := uc.enforceCanReadOrUpdateSanctionCheck(ctx, matchId); err != nil {
		return models.SanctionCheckMatchComment{}, err
	}

	return uc.repository.AddSanctionCheckMatchComment(ctx, uc.executorFactory.NewExecutor(), comment)
}

// Helper functions for enforcing permissions on sanction check actions go below

func (uc SanctionCheckUsecase) enforceCanReadOrUpdateCase(ctx context.Context, decisionId string) (models.Decision, error) {
	exec := uc.executorFactory.NewExecutor()
	decision, err := uc.decisionRepository.DecisionsById(ctx, exec, []string{decisionId})
	if err != nil {
		return models.Decision{}, err
	}
	if len(decision) == 0 {
		return models.Decision{}, errors.Wrap(models.NotFoundError,
			"could not find the decision linked to the sanction check")
	}
	if decision[0].Case == nil {
		return decision[0], errors.Wrap(models.NotFoundError,
			"this sanction check is not linked to a case")
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, decision[0].OrganizationId, false)
	if err != nil {
		return models.Decision{}, errors.Wrap(err,
			"could not retrieve organization inboxes")
	}

	inboxIds := pure_utils.Map(inboxes, func(inbox models.Inbox) string {
		return inbox.Id
	})

	if err := uc.enforceSecurityCase.ReadOrUpdateCase(*decision[0].Case, inboxIds); err != nil {
		return decision[0], err
	}

	return decision[0], nil
}

func (uc SanctionCheckUsecase) enforceCanRefineSanctionCheck(ctx context.Context,
	decisionId string,
) (models.Decision, models.SanctionCheck, error) {
	sanctionCheck, err := uc.repository.GetActiveSanctionCheckForDecision(ctx,
		uc.executorFactory.NewExecutor(), decisionId)
	if err != nil {
		return models.Decision{}, models.SanctionCheck{},
			errors.Wrap(err, "sanction check does not exist")
	}

	if !sanctionCheck.IsReviewable() {
		return models.Decision{}, sanctionCheck,
			errors.Wrap(models.NotFoundError, "this sanction is not pending review")
	}

	decision, err := uc.enforceCanReadOrUpdateCase(ctx, sanctionCheck.DecisionId)
	if err != nil {
		return models.Decision{}, models.SanctionCheck{}, err
	}

	return decision, sanctionCheck, nil
}

func (uc SanctionCheckUsecase) enforceCanReadOrUpdateSanctionCheck(
	ctx context.Context,
	matchId string,
) (models.SanctionCheck, models.SanctionCheckMatch, error) {
	match, err := uc.repository.GetSanctionCheckMatch(ctx, uc.executorFactory.NewExecutor(), matchId)
	if err != nil {
		return models.SanctionCheck{}, models.SanctionCheckMatch{}, err
	}

	sanctionCheck, err := uc.repository.GetSanctionCheck(ctx, uc.executorFactory.NewExecutor(), match.SanctionCheckId)
	if err != nil {
		return sanctionCheck, match, err
	}

	if _, err = uc.enforceCanReadOrUpdateCase(ctx, sanctionCheck.DecisionId); err != nil {
		return sanctionCheck, match, err
	}

	return sanctionCheck, match, nil
}
