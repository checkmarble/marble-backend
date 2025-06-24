package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set/v2"
)

type ScreeningEnforceSecurityScenario interface {
	UpdateScenario(models.Scenario) error
}

type ScreeningEnforceSecurityDecision interface {
	ReadDecision(models.Decision) error
}

type ScreeningEnforceSecurityCase interface {
	ReadOrUpdateCase(models.CaseMetadata, []uuid.UUID) error
}

type ScreeningEnforceSecurity interface {
	ReadWhitelist(context.Context) error
	WriteWhitelist(context.Context) error
	PerformFreeformSearch(context.Context) error
}

type ScreeningProvider interface {
	IsSelfHosted(ctx context.Context) bool
	GetCatalog(ctx context.Context) (models.OpenSanctionsCatalog, error)
	GetLatestLocalDataset(context.Context) (models.OpenSanctionsDatasetFreshness, error)
	Search(context.Context, models.OpenSanctionsQuery) (models.ScreeningRawSearchResponseWithMatches, error)
	EnrichMatch(ctx context.Context, match models.ScreeningMatch) ([]byte, error)
}

type ScreeningInboxReader interface {
	ListInboxes(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		withCaseCount bool,
	) ([]models.Inbox, error)
}

type ScreeningRepository interface {
	GetActiveScreeningForDecision(context.Context, repositories.Executor, string) (
		models.ScreeningWithMatches, error)
	ListScreeningsForDecision(ctx context.Context, exec repositories.Executor, decisionId string, initialOnly bool) (
		[]models.ScreeningWithMatches, error)
	GetScreening(context.Context, repositories.Executor, string) (models.ScreeningWithMatches, error)
	GetScreeningWithoutMatches(context.Context, repositories.Executor, string) (models.Screening, error)
	ArchiveScreening(context.Context, repositories.Executor, string) error
	InsertScreening(
		ctx context.Context,
		exec repositories.Executor,
		decisionid string,
		sc models.ScreeningWithMatches,
		storeMatches bool,
	) (models.ScreeningWithMatches, error)
	UpdateScreeningStatus(ctx context.Context, exec repositories.Executor, id string,
		status models.ScreeningStatus) error

	ListScreeningMatches(ctx context.Context, exec repositories.Executor, screeningId string) (
		[]models.ScreeningMatch, error)
	GetScreeningMatch(ctx context.Context, exec repositories.Executor, matchId string) (models.ScreeningMatch, error)
	UpdateScreeningMatchStatus(ctx context.Context, exec repositories.Executor,
		match models.ScreeningMatch, update models.ScreeningMatchUpdate) (models.ScreeningMatch, error)
	ListScreeningCommentsByIds(ctx context.Context, exec repositories.Executor, ids []string) (
		[]models.ScreeningMatchComment, error)
	AddScreeningMatchComment(ctx context.Context, exec repositories.Executor,
		comment models.ScreeningMatchComment) (models.ScreeningMatchComment, error)
	CreateScreeningFile(ctx context.Context, exec repositories.Executor,
		input models.ScreeningFileInput) (models.ScreeningFile, error)
	GetScreeningFile(ctx context.Context, exec repositories.Executor, matchId, fileId string) (models.ScreeningFile, error)
	ListScreeningFiles(ctx context.Context, exec repositories.Executor, matchId string) ([]models.ScreeningFile, error)
	CopyScreeningFiles(ctx context.Context, exec repositories.Executor,
		screeningId, newScreeningId string) error
	AddScreeningMatchWhitelist(ctx context.Context, exec repositories.Executor,
		orgId, counterpartyId string, entityId string, reviewerId *models.UserId) error
	DeleteScreeningMatchWhitelist(ctx context.Context, exec repositories.Executor,
		orgId string, counterpartyId *string, entityId string, reviewerId *models.UserId) error
	SearchScreeningMatchWhitelist(ctx context.Context, exec repositories.Executor,
		orgId string, counterpartyId, entityId *string) ([]models.ScreeningWhitelist, error)
	IsScreeningMatchWhitelisted(ctx context.Context, exec repositories.Executor,
		orgId, counterpartyId string, entityId []string) ([]models.ScreeningWhitelist, error)
	CountWhitelistsForCounterpartyId(ctx context.Context, exec repositories.Executor,
		orgId, counterpartyId string) (int, error)
	UpdateScreeningMatchPayload(ctx context.Context, exec repositories.Executor,
		match models.ScreeningMatch, newPayload []byte) (models.ScreeningMatch, error)
}

type ScreeningUsecaseExternalRepository interface {
	CreateCaseEvent(
		ctx context.Context,
		exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes,
	) error
	CreateCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) error
	DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error)
}

type ScreeningOrganizationRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
}

type ScreeningCaseUsecase interface {
	PerformCaseActionSideEffects(ctx context.Context, tx repositories.Transaction, c models.Case) error
}

type ScreeningUsecase struct {
	enforceSecurityScenario ScreeningEnforceSecurityScenario
	enforceSecurityDecision ScreeningEnforceSecurityDecision
	enforceSecurityCase     ScreeningEnforceSecurityCase
	enforceSecurity         ScreeningEnforceSecurity

	caseUsecase               ScreeningCaseUsecase
	organizationRepository    ScreeningOrganizationRepository
	externalRepository        ScreeningUsecaseExternalRepository
	screeningConfigRepository ScreeningConfigRepository
	taskQueueRepository       repositories.TaskQueueRepository
	repository                ScreeningRepository

	inboxReader           ScreeningInboxReader
	scenarioFetcher       scenarios.ScenarioFetcher
	openSanctionsProvider ScreeningProvider
	blobBucketUrl         string
	blobRepository        repositories.BlobRepository

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
}

func (uc ScreeningUsecase) CheckDatasetFreshness(ctx context.Context) (models.OpenSanctionsDatasetFreshness, error) {
	return uc.openSanctionsProvider.GetLatestLocalDataset(ctx)
}

func (uc ScreeningUsecase) GetDatasetCatalog(ctx context.Context) (models.OpenSanctionsCatalog, error) {
	return uc.openSanctionsProvider.GetCatalog(ctx)
}

func (uc ScreeningUsecase) GetScreening(ctx context.Context, id string) (models.ScreeningWithMatches, error) {
	sc, err := uc.repository.GetScreening(ctx, uc.executorFactory.NewExecutor(), id)
	if err != nil {
		return models.ScreeningWithMatches{},
			errors.Wrap(err, "could not retrieve screening")
	}

	decisions, err := uc.externalRepository.DecisionsById(ctx, uc.executorFactory.NewExecutor(), []string{sc.DecisionId})
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}
	if len(decisions) == 0 {
		return models.ScreeningWithMatches{},
			errors.WithDetail(models.NotFoundError, "requested decision does not exist")
	}

	if decisions[0].Case == nil {
		if err := uc.enforceSecurityDecision.ReadDecision(decisions[0]); err != nil {
			return models.ScreeningWithMatches{}, err
		}
	} else {
		if _, err = uc.enforceCanReadOrUpdateCase(ctx, decisions[0].DecisionId); err != nil {
			return models.ScreeningWithMatches{}, err
		}
	}

	return sc, nil
}

func (uc ScreeningUsecase) ListScreenings(ctx context.Context, decisionId string,
	initialOnly bool,
) ([]models.ScreeningWithMatches, error) {
	exec := uc.executorFactory.NewExecutor()
	decisions, err := uc.externalRepository.DecisionsById(ctx, exec, []string{decisionId})
	if err != nil {
		return nil, err
	}
	if len(decisions) == 0 {
		return nil, errors.WithDetail(models.NotFoundError, "requested decision does not exist")
	}

	if decisions[0].Case == nil {
		if err := uc.enforceSecurityDecision.ReadDecision(decisions[0]); err != nil {
			return nil, err
		}
	} else {
		if _, err = uc.enforceCanReadOrUpdateCase(ctx, decisions[0].DecisionId); err != nil {
			return nil, err
		}
	}

	scs, err := uc.repository.ListScreeningsForDecision(ctx, exec, decisions[0].DecisionId, initialOnly)
	if err != nil {
		return nil, err
	}

	sccs, err := uc.screeningConfigRepository.ListScreeningConfigs(ctx,
		uc.executorFactory.NewExecutor(), decisions[0].ScenarioIterationId)
	if err != nil {
		return nil, err
	}

	matchIds := set.New[string](0)
	matchIdToMatch := make(map[string]*models.ScreeningMatch)

	var (
		screeningConfig models.ScreeningConfig
		found           bool
	)

	for sidx, sc := range scs {
		for _, scc := range sccs {
			if sc.ScreeningConfigId == scc.Id {
				screeningConfig = scc
				found = true
				break
			}
		}

		if !found {
			return nil, errors.New("could not find screening config for match")
		}

		scs[sidx].Config = models.ScreeningConfigRef{
			Name: screeningConfig.Name,
		}

		for midx, match := range sc.Matches {
			matchIds.Insert(match.Id)
			matchIdToMatch[match.Id] = &scs[sidx].Matches[midx]
		}
	}

	comments, err := uc.repository.ListScreeningCommentsByIds(ctx,
		uc.executorFactory.NewExecutor(), matchIds.Slice())
	if err != nil {
		return nil, err
	}

	for _, comment := range comments {
		if _, ok := matchIdToMatch[comment.MatchId]; ok {
			matchIdToMatch[comment.MatchId].Comments =
				append(matchIdToMatch[comment.MatchId].Comments, comment)
		}
	}

	return scs, nil
}

func (uc ScreeningUsecase) Execute(
	ctx context.Context,
	orgId string,
	query models.OpenSanctionsQuery,
) (models.ScreeningWithMatches, error) {
	org, err := uc.organizationRepository.GetOrganizationById(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return models.ScreeningWithMatches{},
			errors.Wrap(err, "could not retrieve organization")
	}

	query.OrgConfig = org.OpenSanctionsConfig

	matches, err := uc.openSanctionsProvider.Search(ctx, query)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	return matches.AdaptScreeningFromSearchResponse(query), nil
}

func (uc ScreeningUsecase) Refine(ctx context.Context, refine models.ScreeningRefineRequest,
	requestedBy *models.UserId,
) (models.ScreeningWithMatches, error) {
	decision, sc, err := uc.enforceCanRefineScreening(ctx, refine.ScreeningId)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	scc, err := uc.screeningConfigRepository.GetScreeningConfig(ctx,
		uc.executorFactory.NewExecutor(), decision.ScenarioIterationId, sc.ScreeningConfigId)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	query := models.OpenSanctionsQuery{
		IsRefinement: true,
		OrgConfig:    sc.OrgConfig,
		Config:       scc,
		Queries:      models.AdaptRefineRequestToMatchable(refine),
	}

	screening, err := uc.Execute(ctx, decision.OrganizationId, query)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	var requester *string

	if requestedBy != nil {
		requester = utils.Ptr(string(*requestedBy))
	}

	screening.IsManual = true
	screening.RequestedBy = requester

	screening, err = executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (models.ScreeningWithMatches, error) {
		oldScreening, err := uc.repository.GetActiveScreeningForDecision(ctx, tx, sc.Id)
		if err != nil {
			return models.ScreeningWithMatches{}, err
		}

		if err := uc.repository.ArchiveScreening(ctx, tx, sc.Id); err != nil {
			return models.ScreeningWithMatches{}, err
		}

		if screening, err = uc.repository.InsertScreening(ctx, tx,
			decision.DecisionId, screening, true); err != nil {
			return models.ScreeningWithMatches{}, err
		}

		if uc.openSanctionsProvider.IsSelfHosted(ctx) {
			if err := uc.taskQueueRepository.EnqueueMatchEnrichmentTask(ctx,
				tx, decision.OrganizationId, screening.Id); err != nil {
				utils.LogAndReportSentryError(ctx, errors.Wrap(err,
					"could not enqueue screening for refinement"))
			}
		}

		if err := uc.repository.CopyScreeningFiles(ctx, tx, oldScreening.Id, screening.Id); err != nil {
			return screening, errors.Wrap(err,
				"could not copy screening uploaded files for refinement")
		}

		return screening, err
	})
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	return screening, nil
}

func (uc ScreeningUsecase) Search(ctx context.Context, refine models.ScreeningRefineRequest) (models.ScreeningWithMatches, error) {
	decision, sc, err := uc.enforceCanRefineScreening(ctx, refine.ScreeningId)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	scc, err := uc.screeningConfigRepository.GetScreeningConfig(ctx,
		uc.executorFactory.NewExecutor(), decision.ScenarioIterationId, sc.ScreeningConfigId)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	query := models.OpenSanctionsQuery{
		IsRefinement: true,
		OrgConfig:    sc.OrgConfig,
		Config:       scc,
		Queries:      models.AdaptRefineRequestToMatchable(refine),
	}

	screening, err := uc.Execute(ctx, decision.OrganizationId, query)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	return screening, nil
}

func (uc ScreeningUsecase) FreeformSearch(ctx context.Context,
	orgId string,
	scc models.ScreeningConfig,
	refine models.ScreeningRefineRequest,
) (models.ScreeningWithMatches, error) {
	if err := uc.enforceSecurity.PerformFreeformSearch(ctx); err != nil {
		return models.ScreeningWithMatches{}, err
	}

	query := models.OpenSanctionsQuery{
		IsRefinement: false,
		Config:       scc,
		Queries:      models.AdaptRefineRequestToMatchable(refine),
	}

	screening, err := uc.Execute(ctx, orgId, query)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	return screening, nil
}

func (uc ScreeningUsecase) FilterOutWhitelistedMatches(ctx context.Context, orgId string,
	screening models.ScreeningWithMatches, counterpartyId string,
) (models.ScreeningWithMatches, error) {
	matchesSet := set.From(pure_utils.FlatMap(screening.Matches, func(m models.ScreeningMatch) []string {
		return append(m.Referents, m.EntityId)
	}))

	whitelists, err := uc.repository.IsScreeningMatchWhitelisted(ctx,
		uc.executorFactory.NewExecutor(), orgId, counterpartyId, matchesSet.Slice())
	if err != nil {
		return screening, err
	}

	matchesAfterWhitelisting := make([]models.ScreeningMatch, 0, len(screening.Matches))

	for _, match := range screening.Matches {
		isWhitelisted := slices.ContainsFunc(whitelists, func(w models.ScreeningWhitelist) bool {
			if match.EntityId == w.EntityId {
				return true
			}

			return slices.Contains(match.Referents, w.EntityId)
		})

		if isWhitelisted {
			continue
		}

		matchesAfterWhitelisting = append(matchesAfterWhitelisting, match)
	}

	if len(whitelists) > 0 {
		utils.LoggerFromContext(ctx).InfoContext(ctx,
			"filtered out screening matches that were whitelisted", "before",
			len(screening.Matches), "whitelisted", len(whitelists), "after", len(matchesAfterWhitelisting))

		whitelisted := pure_utils.Map(whitelists, func(w models.ScreeningWhitelist) string {
			return w.EntityId
		})

		screening.WhitelistedEntities = whitelisted
	}

	screening.Matches = matchesAfterWhitelisting
	screening.Count = len(screening.Matches)

	if screening.Count == 0 {
		screening.Status = models.ScreeningStatusNoHit
	}

	return screening, nil
}

func (uc ScreeningUsecase) CountWhitelistsForCounterpartyId(ctx context.Context, orgId, counterpartyId string) (int, error) {
	return uc.repository.CountWhitelistsForCounterpartyId(ctx, uc.executorFactory.NewExecutor(), orgId, counterpartyId)
}

func (uc ScreeningUsecase) UpdateMatchStatus(
	ctx context.Context,
	update models.ScreeningMatchUpdate,
) (models.ScreeningMatch, error) {
	data, err := uc.enforceCanReadOrUpdateScreeningMatch(ctx, update.MatchId)
	if err != nil {
		return models.ScreeningMatch{}, err
	}

	if update.Status != models.ScreeningMatchStatusConfirmedHit &&
		update.Status != models.ScreeningMatchStatusNoHit {
		return data.match, errors.Wrap(models.BadParameterError,
			"invalid status received for screening match, should be 'confirmed_hit' or 'no_hit'")
	}

	if !data.sanction.Status.IsReviewable() {
		return data.match, errors.WithDetail(models.UnprocessableEntityError,
			"this sanction is not pending review")
	}

	if data.match.Status != models.ScreeningMatchStatusPending {
		return data.match, errors.WithDetail(models.UnprocessableEntityError, "this match is not pending review")
	}

	var updatedMatch models.ScreeningMatch
	err = uc.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			allMatches, err := uc.repository.ListScreeningMatches(ctx, tx, data.sanction.Id)
			if err != nil {
				return err
			}
			pendingMatchesExcludingThis := utils.Filter(allMatches, func(m models.ScreeningMatch) bool {
				return m.Id != data.match.Id && m.Status == models.ScreeningMatchStatusPending
			})

			updatedMatch, err = uc.repository.UpdateScreeningMatchStatus(ctx, tx, data.match, update)
			if err != nil {
				return err
			}

			if update.Comment != nil {
				comment, err := uc.MatchAddComment(ctx, data.match.Id, *update.Comment)
				if err != nil {
					return errors.Wrap(err, "could not add comment while updating match status")
				}

				// For now, there can be only one comment, added while reviewing, so we can directly add it.
				updatedMatch.Comments = []models.ScreeningMatchComment{comment}
			}

			if data.decision.Case != nil {
				if err := uc.caseUsecase.PerformCaseActionSideEffects(ctx, tx, *data.decision.Case); err != nil {
					return err
				}
			}

			// If the match is confirmed, all other pending matches should be set to "skipped" and the screening to "confirmed_hit"
			if update.Status == models.ScreeningMatchStatusConfirmedHit {
				for _, m := range pendingMatchesExcludingThis {
					_, err = uc.repository.UpdateScreeningMatchStatus(ctx, tx, m, models.ScreeningMatchUpdate{
						MatchId:    m.Id,
						Status:     models.ScreeningMatchStatusSkipped,
						ReviewerId: update.ReviewerId,
					})
					if err != nil {
						return err
					}

				}

				err = uc.repository.UpdateScreeningStatus(ctx, tx, data.sanction.Id, models.ScreeningStatusConfirmedHit)
				if err != nil {
					return err
				}

				if data.decision.Case != nil {
					var reviewerId *string

					if update.ReviewerId != nil && len(*update.ReviewerId) > 0 {
						reviewerId = utils.Ptr(string(*update.ReviewerId))
					}

					err = uc.externalRepository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
						CaseId:       data.decision.Case.Id,
						UserId:       reviewerId,
						EventType:    models.ScreeningReviewed,
						ResourceId:   &data.decision.DecisionId,
						ResourceType: utils.Ptr(models.DecisionResourceType),
						NewValue:     utils.Ptr(models.ScreeningMatchStatusConfirmedHit.String()),
					})
					if err != nil {
						return err
					}
				}
			}

			// else, if it is the last match pending and it is not a hit, the screening should be set to "no_hit"
			if update.Status == models.ScreeningMatchStatusNoHit && len(pendingMatchesExcludingThis) == 0 {
				err = uc.repository.UpdateScreeningStatus(ctx, tx,
					data.sanction.Id, models.ScreeningStatusNoHit)
				if err != nil {
					return err
				}

				if data.decision.Case != nil {
					var reviewerId *string

					if update.ReviewerId != nil && len(*update.ReviewerId) > 0 {
						reviewerId = utils.Ptr(string(*update.ReviewerId))
					}

					err = uc.externalRepository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
						CaseId:       data.decision.Case.Id,
						UserId:       reviewerId,
						EventType:    models.ScreeningReviewed,
						ResourceId:   &data.decision.DecisionId,
						ResourceType: utils.Ptr(models.DecisionResourceType),
						NewValue:     utils.Ptr(models.ScreeningMatchStatusNoHit.String()),
					})
					if err != nil {
						return err
					}
				}
			}

			if update.Status == models.ScreeningMatchStatusNoHit && update.Whitelist &&
				data.match.UniqueCounterpartyIdentifier != nil {
				if err := uc.CreateWhitelist(ctx, tx,
					data.decision.OrganizationId, *data.match.UniqueCounterpartyIdentifier,
					data.match.EntityId, update.ReviewerId); err != nil {
					return errors.Wrap(err, "could not whitelist match")
				}
			}

			return nil
		},
	)

	return updatedMatch, err
}

func (uc ScreeningUsecase) CreateWhitelist(ctx context.Context, exec repositories.Executor,
	orgId, counterpartyId, entityId string, reviewerId *models.UserId,
) error {
	if err := uc.enforceSecurity.WriteWhitelist(ctx); err != nil {
		return err
	}

	if exec == nil {
		exec = uc.executorFactory.NewExecutor()
	}

	if err := uc.repository.AddScreeningMatchWhitelist(ctx, exec, orgId, counterpartyId, entityId, reviewerId); err != nil {
		return err
	}

	return nil
}

func (uc ScreeningUsecase) DeleteWhitelist(ctx context.Context, exec repositories.Executor,
	orgId string, counterpartyId *string, entityId string, reviewerId *models.UserId,
) error {
	if err := uc.enforceSecurity.WriteWhitelist(ctx); err != nil {
		return err
	}

	if exec == nil {
		exec = uc.executorFactory.NewExecutor()
	}

	if err := uc.repository.DeleteScreeningMatchWhitelist(ctx, exec, orgId, counterpartyId, entityId, reviewerId); err != nil {
		return err
	}

	return nil
}

func (uc ScreeningUsecase) SearchWhitelist(ctx context.Context, exec repositories.Executor,
	orgId string, counterpartyId, entityId *string, reviewerId *models.UserId,
) ([]models.ScreeningWhitelist, error) {
	if err := uc.enforceSecurity.ReadWhitelist(ctx); err != nil {
		return nil, err
	}

	if exec == nil {
		exec = uc.executorFactory.NewExecutor()
	}

	whitelists, err := uc.repository.SearchScreeningMatchWhitelist(ctx, exec, orgId, counterpartyId, entityId)
	if err != nil {
		return nil, err
	}

	return whitelists, nil
}

func (uc ScreeningUsecase) MatchAddComment(ctx context.Context, matchId string,
	comment models.ScreeningMatchComment,
) (models.ScreeningMatchComment, error) {
	if _, err := uc.enforceCanReadOrUpdateScreeningMatch(ctx, matchId); err != nil {
		return models.ScreeningMatchComment{}, err
	}

	return uc.repository.AddScreeningMatchComment(ctx, uc.executorFactory.NewExecutor(), comment)
}

func (uc ScreeningUsecase) EnrichMatch(ctx context.Context, matchId string) (models.ScreeningMatch, error) {
	if _, err := uc.enforceCanReadOrUpdateScreeningMatch(ctx, matchId); err != nil {
		return models.ScreeningMatch{}, err
	}

	return uc.EnrichMatchWithoutAuthorization(ctx, matchId)
}

func (uc ScreeningUsecase) EnrichMatchWithoutAuthorization(ctx context.Context, matchId string) (models.ScreeningMatch, error) {
	match, err := uc.repository.GetScreeningMatch(ctx, uc.executorFactory.NewExecutor(), matchId)
	if err != nil {
		return models.ScreeningMatch{}, err
	}

	if match.Enriched {
		return models.ScreeningMatch{}, errors.WithDetail(models.UnprocessableEntityError,
			"this screening match was already enriched")
	}

	newPayload, err := uc.openSanctionsProvider.EnrichMatch(ctx, match)
	if err != nil {
		return models.ScreeningMatch{}, err
	}

	mergedPayload, err := mergePayloads(match.Payload, newPayload)
	if err != nil {
		return models.ScreeningMatch{}, errors.Wrap(err,
			"could not merge payloads for match enrichment")
	}

	newMatch, err := uc.repository.UpdateScreeningMatchPayload(ctx,
		uc.executorFactory.NewExecutor(), match, mergedPayload)
	if err != nil {
		return models.ScreeningMatch{}, err
	}

	return newMatch, nil
}

func (uc ScreeningUsecase) GetEntity(ctx context.Context, entityId string) ([]byte, error) {
	return uc.openSanctionsProvider.EnrichMatch(ctx, models.ScreeningMatch{EntityId: entityId})
}

func (uc ScreeningUsecase) CreateFiles(ctx context.Context, creds models.Credentials,
	screeningId string, files []multipart.FileHeader,
) ([]models.ScreeningFile, error) {
	sc, err := uc.repository.GetActiveScreeningForDecision(ctx,
		uc.executorFactory.NewExecutor(), screeningId)
	if err != nil {
		return nil, err
	}

	match, err := uc.enforceCanReadOrUpdateCase(ctx, sc.DecisionId)
	if err != nil {
		return nil, err
	}

	for _, fileHeader := range files {
		if err := validateFileType(fileHeader); err != nil {
			return nil, err
		}
	}

	type uploadedFileMetadata struct {
		fileReference string
		fileName      string
	}

	metadata := make([]uploadedFileMetadata, 0, len(files))

	for _, fileHeader := range files {
		newFileReference := fmt.Sprintf("%s/%s/%s", creds.OrganizationId, sc.Id, uuid.NewString())
		err = writeScreeningFileToBlobStorage(ctx, uc.blobRepository, uc.blobBucketUrl, fileHeader, newFileReference)
		if err != nil {
			break
		}

		metadata = append(metadata, uploadedFileMetadata{
			fileReference: newFileReference,
			fileName:      fileHeader.Filename,
		})
	}

	logger := utils.LoggerFromContext(ctx)

	if err != nil {
		for _, uploadedFile := range metadata {
			if deleteErr := uc.blobRepository.DeleteFile(ctx, uc.blobBucketUrl,
				uploadedFile.fileReference); deleteErr != nil {
				logger.WarnContext(ctx, fmt.Sprintf("failed to clean up blob %s after case file creation failed", uploadedFile.fileReference),
					"bucket", uc.blobBucketUrl,
					"file_reference", uploadedFile.fileReference,
					"error", deleteErr)
			}
		}

		return nil, err
	}

	uploadedFiles := make([]models.ScreeningFile, len(metadata))

	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		for idx, uploadedFile := range metadata {
			file, err := uc.repository.CreateScreeningFile(
				ctx,
				tx,
				models.ScreeningFileInput{
					BucketName:    uc.blobBucketUrl,
					ScreeningId:   sc.Id,
					FileName:      uploadedFile.fileName,
					FileReference: uploadedFile.fileReference,
				},
			)
			if err != nil {
				return err
			}

			uploadedFiles[idx] = file
		}

		if err := uc.externalRepository.CreateCaseContributor(ctx, tx, match.Case.Id,
			string(creds.ActorIdentity.UserId)); err != nil {
			return errors.Wrap(err, "could not create case contributor for screening file upload")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return uploadedFiles, nil
}

func (uc ScreeningUsecase) ListFiles(ctx context.Context, screeningId string) ([]models.ScreeningFile, error) {
	sc, err := uc.repository.GetActiveScreeningForDecision(ctx,
		uc.executorFactory.NewExecutor(), screeningId)
	if err != nil {
		return nil, err
	}

	if _, err := uc.enforceCanReadOrUpdateCase(ctx, sc.DecisionId); err != nil {
		return nil, err
	}

	files, err := uc.repository.ListScreeningFiles(ctx, uc.executorFactory.NewExecutor(), sc.Id)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (uc ScreeningUsecase) GetFileDownloadUrl(ctx context.Context, screeningId, fileId string) (string, error) {
	sc, err := uc.repository.GetScreening(ctx, uc.executorFactory.NewExecutor(), screeningId)
	if err != nil {
		return "", err
	}

	if _, err := uc.enforceCanReadOrUpdateCase(ctx, sc.DecisionId); err != nil {
		return "", err
	}

	file, err := uc.repository.GetScreeningFile(ctx, uc.executorFactory.NewExecutor(), sc.Id, fileId)
	if err != nil {
		return "", err
	}

	return uc.blobRepository.GenerateSignedUrl(ctx, uc.blobBucketUrl, file.FileReference)
}

func writeScreeningFileToBlobStorage(ctx context.Context,
	blobRepository repositories.BlobRepository, bucketUrl string,
	fileHeader multipart.FileHeader, newFileReference string,
) error {
	writer, err := blobRepository.OpenStream(ctx, bucketUrl, newFileReference, fileHeader.Filename)
	if err != nil {
		return err
	}
	defer writer.Close() // We should still call Close when we are finished writing to check the error if any - this is a no-op if Close has already been called

	file, err := fileHeader.Open()
	if err != nil {
		return errors.Wrap(models.BadParameterError, err.Error())
	}
	if _, err := io.Copy(writer, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return nil
}

// Helper functions for enforcing permissions on screening actions go below

func (uc ScreeningUsecase) enforceCanReadOrUpdateCase(ctx context.Context, decisionId string) (models.Decision, error) {
	creds, _ := utils.CredentialsFromCtx(ctx)

	exec := uc.executorFactory.NewExecutor()
	decision, err := uc.externalRepository.DecisionsById(ctx, exec, []string{decisionId})
	if err != nil {
		return models.Decision{}, err
	}
	if len(decision) == 0 {
		return models.Decision{}, errors.Wrap(models.NotFoundError,
			"could not find the decision linked to the screening")
	}

	if creds.Role != models.API_CLIENT {
		if decision[0].Case == nil {
			return decision[0], errors.Wrap(models.UnprocessableEntityError,
				"this screening is not linked to a case")
		}

		inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, decision[0].OrganizationId, false)
		if err != nil {
			return models.Decision{}, errors.Wrap(err,
				"could not retrieve organization inboxes")
		}

		inboxIds := pure_utils.Map(inboxes, func(inbox models.Inbox) uuid.UUID {
			return inbox.Id
		})

		if err := uc.enforceSecurityCase.ReadOrUpdateCase((*decision[0].Case).GetMetadata(), inboxIds); err != nil {
			return decision[0], err
		}
	}

	return decision[0], nil
}

func (uc ScreeningUsecase) enforceCanRefineScreening(
	ctx context.Context,
	screeningId string,
) (models.Decision, models.ScreeningWithMatches, error) {
	sc, err := uc.repository.GetScreening(ctx, uc.executorFactory.NewExecutor(), screeningId)
	if err != nil {
		return models.Decision{}, models.ScreeningWithMatches{},
			errors.WithDetail(err, "screening does not exist")
	}

	if !sc.Status.IsRefinable() {
		return models.Decision{}, sc,
			errors.WithDetail(models.NotFoundError,
				"this sanction is not pending review or error")
	}

	decision, err := uc.enforceCanReadOrUpdateCase(ctx, sc.DecisionId)
	if err != nil {
		return models.Decision{}, models.ScreeningWithMatches{}, err
	}

	return decision, sc, nil
}

type screeningContextData struct {
	decision models.Decision
	sanction models.Screening
	match    models.ScreeningMatch
}

func (uc ScreeningUsecase) enforceCanReadOrUpdateScreeningMatch(
	ctx context.Context,
	matchId string,
) (screeningContextData, error) {
	match, err := uc.repository.GetScreeningMatch(ctx, uc.executorFactory.NewExecutor(), matchId)
	if err != nil {
		return screeningContextData{}, err
	}

	screening, err := uc.repository.GetScreeningWithoutMatches(ctx,
		uc.executorFactory.NewExecutor(), match.ScreeningId)
	if err != nil {
		return screeningContextData{}, err
	}

	if screening.IsArchived {
		return screeningContextData{}, errors.WithDetail(models.UnprocessableEntityError,
			"screening was refined and cannot be reviewed")
	}

	dec, err := uc.enforceCanReadOrUpdateCase(ctx, screening.DecisionId)
	if err != nil {
		return screeningContextData{}, err
	}

	return screeningContextData{
		decision: dec,
		sanction: screening,
		match:    match,
	}, nil
}

func mergePayloads(originalRaw, newRaw []byte) ([]byte, error) {
	var original, new map[string]any

	if err := json.Unmarshal(originalRaw, &original); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newRaw, &new); err != nil {
		return nil, err
	}

	maps.Copy(original, new)

	out, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}

	return out, nil
}
