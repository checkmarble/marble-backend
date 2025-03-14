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

type SanctionCheckEnforceSecurityScenario interface {
	UpdateScenario(models.Scenario) error
}

type SanctionCheckEnforceSecurityDecision interface {
	ReadDecision(models.Decision) error
}

type SanctionCheckEnforceSecurityCase interface {
	ReadOrUpdateCase(models.Case, []string) error
}

type SanctionCheckProvider interface {
	IsSelfHosted(ctx context.Context) bool
	GetCatalog(ctx context.Context) (models.OpenSanctionsCatalog, error)
	GetLatestLocalDataset(context.Context) (models.OpenSanctionsDatasetFreshness, error)
	Search(context.Context, models.OpenSanctionsQuery) (models.SanctionRawSearchResponseWithMatches, error)
	EnrichMatch(ctx context.Context, match models.SanctionCheckMatch) ([]byte, error)
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
	GetActiveSanctionCheckForDecision(context.Context, repositories.Executor, string) (
		*models.SanctionCheckWithMatches, error)
	GetSanctionChecksForDecision(ctx context.Context, exec repositories.Executor, decisionId string, initialOnly bool) (
		[]models.SanctionCheckWithMatches, error)
	GetSanctionCheck(context.Context, repositories.Executor, string) (models.SanctionCheckWithMatches, error)
	GetSanctionCheckWithoutMatches(context.Context, repositories.Executor, string) (models.SanctionCheck, error)
	ArchiveSanctionCheck(context.Context, repositories.Executor, string) error
	InsertSanctionCheck(
		ctx context.Context,
		exec repositories.Executor,
		decisionid string,
		sc models.SanctionCheckWithMatches,
		storeMatches bool,
	) (models.SanctionCheckWithMatches, error)
	UpdateSanctionCheckStatus(ctx context.Context, exec repositories.Executor, id string,
		status models.SanctionCheckStatus) error

	ListSanctionCheckMatches(ctx context.Context, exec repositories.Executor, sanctionCheckId string) (
		[]models.SanctionCheckMatch, error)
	GetSanctionCheckMatch(ctx context.Context, exec repositories.Executor, matchId string) (models.SanctionCheckMatch, error)
	UpdateSanctionCheckMatchStatus(ctx context.Context, exec repositories.Executor,
		match models.SanctionCheckMatch, update models.SanctionCheckMatchUpdate) (models.SanctionCheckMatch, error)
	ListSanctionCheckCommentsByIds(ctx context.Context, exec repositories.Executor, ids []string) (
		[]models.SanctionCheckMatchComment, error)
	AddSanctionCheckMatchComment(ctx context.Context, exec repositories.Executor,
		comment models.SanctionCheckMatchComment) (models.SanctionCheckMatchComment, error)
	CreateSanctionCheckFile(ctx context.Context, exec repositories.Executor,
		input models.SanctionCheckFileInput) (models.SanctionCheckFile, error)
	GetSanctionCheckFile(ctx context.Context, exec repositories.Executor, matchId, fileId string) (models.SanctionCheckFile, error)
	ListSanctionCheckFiles(ctx context.Context, exec repositories.Executor, matchId string) ([]models.SanctionCheckFile, error)
	CopySanctionCheckFiles(ctx context.Context, exec repositories.Executor,
		sanctionCheckId, newSanctionCheckId string) error
	AddSanctionCheckMatchWhitelist(ctx context.Context, exec repositories.Executor,
		orgId, counterpartyId string, entityId string, reviewerId *models.UserId) error
	DeleteSanctionCheckMatchWhitelist(ctx context.Context, exec repositories.Executor,
		orgId string, counterpartyId *string, entityId string, reviewerId *models.UserId) error
	SearchSanctionCheckMatchWhitelist(ctx context.Context, exec repositories.Executor,
		orgId string, counterpartyId, entityId *string) ([]models.SanctionCheckWhitelist, error)
	IsSanctionCheckMatchWhitelisted(ctx context.Context, exec repositories.Executor,
		orgId, counterpartyId string, entityId []string) ([]models.SanctionCheckWhitelist, error)
	CountWhitelistsForCounterpartyId(ctx context.Context, exec repositories.Executor,
		orgId, counterpartyId string) (int, error)
	UpdateSanctionCheckMatchPayload(ctx context.Context, exec repositories.Executor,
		match models.SanctionCheckMatch, newPayload []byte) (models.SanctionCheckMatch, error)
}

type SanctionsCheckUsecaseExternalRepository interface {
	CreateCaseEvent(
		ctx context.Context,
		exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes,
	) error
	CreateCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) error
	DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error)
}

type SanctionCheckOrganizationRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
}

type SanctionCheckUsecase struct {
	enforceSecurityScenario SanctionCheckEnforceSecurityScenario
	enforceSecurityDecision SanctionCheckEnforceSecurityDecision
	enforceSecurityCase     SanctionCheckEnforceSecurityCase

	organizationRepository        SanctionCheckOrganizationRepository
	externalRepository            SanctionsCheckUsecaseExternalRepository
	sanctionCheckConfigRepository SanctionCheckConfigRepository
	taskQueueRepository           repositories.TaskQueueRepository
	repository                    SanctionCheckRepository

	inboxReader           SanctionCheckInboxReader
	scenarioFetcher       scenarios.ScenarioFetcher
	openSanctionsProvider SanctionCheckProvider
	blobBucketUrl         string
	blobRepository        repositories.BlobRepository

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
}

func (uc SanctionCheckUsecase) CheckDatasetFreshness(ctx context.Context) (models.OpenSanctionsDatasetFreshness, error) {
	return uc.openSanctionsProvider.GetLatestLocalDataset(ctx)
}

func (uc SanctionCheckUsecase) GetDatasetCatalog(ctx context.Context) (models.OpenSanctionsCatalog, error) {
	return uc.openSanctionsProvider.GetCatalog(ctx)
}

func (uc SanctionCheckUsecase) ListSanctionChecks(ctx context.Context, decisionId string,
	initialOnly bool,
) ([]models.SanctionCheckWithMatches, error) {
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

	scs, err := uc.repository.GetSanctionChecksForDecision(ctx, exec, decisions[0].DecisionId, initialOnly)
	if err != nil {
		return nil, err
	}

	scc, err := uc.sanctionCheckConfigRepository.GetSanctionCheckConfig(ctx,
		uc.executorFactory.NewExecutor(), decisions[0].ScenarioIterationId)
	if err != nil {
		return nil, err
	}

	matchIds := set.New[string](0)
	matchIdToMatch := make(map[string]*models.SanctionCheckMatch)

	for sidx, sc := range scs {
		scs[sidx].Config = models.SanctionCheckConfigRef{
			Name: scc.Name,
		}

		for midx, match := range sc.Matches {
			matchIds.Insert(match.Id)
			matchIdToMatch[match.Id] = &scs[sidx].Matches[midx]
		}
	}

	comments, err := uc.repository.ListSanctionCheckCommentsByIds(ctx,
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

func (uc SanctionCheckUsecase) Execute(
	ctx context.Context,
	orgId string,
	query models.OpenSanctionsQuery,
) (models.SanctionCheckWithMatches, error) {
	org, err := uc.organizationRepository.GetOrganizationById(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return models.SanctionCheckWithMatches{},
			errors.Wrap(err, "could not retrieve organization")
	}

	query.OrgConfig = org.OpenSanctionsConfig

	matches, err := uc.openSanctionsProvider.Search(ctx, query)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	return matches.AdaptSanctionCheckFromSearchResponse(query), nil
}

func (uc SanctionCheckUsecase) Refine(ctx context.Context, refine models.SanctionCheckRefineRequest,
	requestedBy *models.UserId,
) (models.SanctionCheckWithMatches, error) {
	decision, sc, err := uc.enforceCanRefineSanctionCheck(ctx, refine.DecisionId)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	scc, err := uc.sanctionCheckConfigRepository.GetSanctionCheckConfig(ctx,
		uc.executorFactory.NewExecutor(), decision.ScenarioIterationId)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	query := models.OpenSanctionsQuery{
		IsRefinement: true,
		OrgConfig:    sc.OrgConfig,
		Config:       *scc,
		Queries:      models.AdaptRefineRequestToMatchable(refine),
	}

	sanctionCheck, err := uc.Execute(ctx, decision.OrganizationId, query)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	var requester *string

	if requestedBy != nil {
		requester = utils.Ptr(string(*requestedBy))
	}

	sanctionCheck.IsManual = true
	sanctionCheck.RequestedBy = requester

	sanctionCheck, err = executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (models.SanctionCheckWithMatches, error) {
		oldSanctionCheck, err := uc.repository.GetActiveSanctionCheckForDecision(ctx, tx, decision.DecisionId)
		if err != nil {
			return models.SanctionCheckWithMatches{}, err
		}

		if err := uc.repository.ArchiveSanctionCheck(ctx, tx, decision.DecisionId); err != nil {
			return models.SanctionCheckWithMatches{}, err
		}

		if sanctionCheck, err = uc.repository.InsertSanctionCheck(ctx, tx,
			decision.DecisionId, sanctionCheck, true); err != nil {
			return models.SanctionCheckWithMatches{}, err
		}

		if uc.openSanctionsProvider.IsSelfHosted(ctx) {
			if err := uc.taskQueueRepository.EnqueueMatchEnrichmentTask(ctx,
				decision.OrganizationId, sanctionCheck.Id); err != nil {
				utils.LogAndReportSentryError(ctx, errors.Wrap(err,
					"could not enqueue sanction check for refinement"))
			}
		}

		if err := uc.repository.CopySanctionCheckFiles(ctx, tx, oldSanctionCheck.Id, sanctionCheck.Id); err != nil {
			return sanctionCheck, errors.Wrap(err,
				"could not copy sanction check uploaded files for refinement")
		}

		return sanctionCheck, err
	})
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	return sanctionCheck, nil
}

func (uc SanctionCheckUsecase) Search(ctx context.Context, refine models.SanctionCheckRefineRequest) (models.SanctionCheckWithMatches, error) {
	decision, sc, err := uc.enforceCanRefineSanctionCheck(ctx, refine.DecisionId)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	scc, err := uc.sanctionCheckConfigRepository.GetSanctionCheckConfig(ctx,
		uc.executorFactory.NewExecutor(), decision.ScenarioIterationId)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	query := models.OpenSanctionsQuery{
		IsRefinement: true,
		OrgConfig:    sc.OrgConfig,
		Config:       *scc,
		Queries:      models.AdaptRefineRequestToMatchable(refine),
	}

	sanctionCheck, err := uc.Execute(ctx, decision.OrganizationId, query)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	return sanctionCheck, nil
}

func (uc SanctionCheckUsecase) FilterOutWhitelistedMatches(ctx context.Context, orgId string,
	sanctionCheck models.SanctionCheckWithMatches, counterpartyId string,
) (models.SanctionCheckWithMatches, error) {
	matchesSet := set.From(pure_utils.Map(sanctionCheck.Matches, func(m models.SanctionCheckMatch) string {
		return m.EntityId
	}))

	whitelists, err := uc.repository.IsSanctionCheckMatchWhitelisted(ctx,
		uc.executorFactory.NewExecutor(), orgId, counterpartyId, matchesSet.Slice())
	if err != nil {
		return sanctionCheck, err
	}

	matchesAfterWhitelisting := make([]models.SanctionCheckMatch, 0, len(sanctionCheck.Matches))

	for _, match := range sanctionCheck.Matches {
		isWhitelisted := slices.ContainsFunc(whitelists, func(w models.SanctionCheckWhitelist) bool {
			return match.EntityId == w.EntityId
		})

		if isWhitelisted {
			continue
		}

		matchesAfterWhitelisting = append(matchesAfterWhitelisting, match)
	}

	if len(whitelists) > 0 {
		utils.LoggerFromContext(ctx).InfoContext(ctx,
			"filtered out sanction check matches that were whitelisted", "before",
			len(sanctionCheck.Matches), "whitelisted", len(whitelists), "after", len(matchesAfterWhitelisting))

		whitelisted := pure_utils.Map(whitelists, func(w models.SanctionCheckWhitelist) string {
			return w.EntityId
		})

		sanctionCheck.WhitelistedEntities = whitelisted
	}

	sanctionCheck.Matches = matchesAfterWhitelisting
	sanctionCheck.Count = len(sanctionCheck.Matches)

	return sanctionCheck, nil
}

func (uc SanctionCheckUsecase) CountWhitelistsForCounterpartyId(ctx context.Context, orgId, counterpartyId string) (int, error) {
	return uc.repository.CountWhitelistsForCounterpartyId(ctx, uc.executorFactory.NewExecutor(), orgId, counterpartyId)
}

func (uc SanctionCheckUsecase) UpdateMatchStatus(
	ctx context.Context,
	update models.SanctionCheckMatchUpdate,
) (models.SanctionCheckMatch, error) {
	data, err := uc.enforceCanReadOrUpdateSanctionCheckMatch(ctx, update.MatchId)
	if err != nil {
		return models.SanctionCheckMatch{}, err
	}

	if update.Status != models.SanctionMatchStatusConfirmedHit &&
		update.Status != models.SanctionMatchStatusNoHit {
		return data.match, errors.Wrap(models.BadParameterError,
			"invalid status received for sanction check match, should be 'confirmed_hit' or 'no_hit'")
	}

	if !data.sanction.Status.IsReviewable() {
		return data.match, errors.WithDetail(models.UnprocessableEntityError,
			"this sanction is not pending review")
	}

	if data.match.Status != models.SanctionMatchStatusPending {
		return data.match, errors.WithDetail(models.UnprocessableEntityError, "this match is not pending review")
	}

	var updatedMatch models.SanctionCheckMatch
	err = uc.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			allMatches, err := uc.repository.ListSanctionCheckMatches(ctx, tx, data.sanction.Id)
			if err != nil {
				return err
			}
			pendingMatchesExcludingThis := utils.Filter(allMatches, func(m models.SanctionCheckMatch) bool {
				return m.Id != data.match.Id && m.Status == models.SanctionMatchStatusPending
			})

			updatedMatch, err = uc.repository.UpdateSanctionCheckMatchStatus(ctx, tx, data.match, update)
			if err != nil {
				return err
			}

			if update.Comment != nil {
				comment, err := uc.MatchAddComment(ctx, data.match.Id, *update.Comment)
				if err != nil {
					return errors.Wrap(err, "could not add comment while updating match status")
				}

				// For now, there can be only one comment, added while reviewing, so we can directly add it.
				updatedMatch.Comments = []models.SanctionCheckMatchComment{comment}
			}

			// If the match is confirmed, all other pending matches should be set to "skipped" and the sanction check to "confirmed_hit"
			if update.Status == models.SanctionMatchStatusConfirmedHit {
				for _, m := range pendingMatchesExcludingThis {
					_, err = uc.repository.UpdateSanctionCheckMatchStatus(ctx, tx, m, models.SanctionCheckMatchUpdate{
						MatchId:    m.Id,
						Status:     models.SanctionMatchStatusSkipped,
						ReviewerId: update.ReviewerId,
					})
					if err != nil {
						return err
					}

				}

				err = uc.repository.UpdateSanctionCheckStatus(ctx, tx, data.sanction.Id, models.SanctionStatusConfirmedHit)
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
						EventType:    models.SanctionCheckReviewed,
						ResourceId:   &data.decision.DecisionId,
						ResourceType: utils.Ptr(models.DecisionResourceType),
						NewValue:     utils.Ptr(models.SanctionMatchStatusConfirmedHit.String()),
					})
					if err != nil {
						return err
					}
				}
			}

			// else, if it is the last match pending and it is not a hit, the sanction check should be set to "no_hit"
			if update.Status == models.SanctionMatchStatusNoHit && len(pendingMatchesExcludingThis) == 0 {
				err = uc.repository.UpdateSanctionCheckStatus(ctx, tx,
					data.sanction.Id, models.SanctionStatusNoHit)
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
						EventType:    models.SanctionCheckReviewed,
						ResourceId:   &data.decision.DecisionId,
						ResourceType: utils.Ptr(models.DecisionResourceType),
						NewValue:     utils.Ptr(models.SanctionMatchStatusNoHit.String()),
					})
					if err != nil {
						return err
					}
				}
			}

			if update.Status == models.SanctionMatchStatusNoHit && update.Whitelist &&
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

func (uc SanctionCheckUsecase) CreateWhitelist(ctx context.Context, exec repositories.Executor,
	orgId, counterpartyId, entityId string, reviewerId *models.UserId,
) error {
	if exec == nil {
		exec = uc.executorFactory.NewExecutor()
	}

	if err := uc.repository.AddSanctionCheckMatchWhitelist(ctx, exec, orgId, counterpartyId, entityId, reviewerId); err != nil {
		return err
	}

	return nil
}

func (uc SanctionCheckUsecase) DeleteWhitelist(ctx context.Context, exec repositories.Executor,
	orgId string, counterpartyId *string, entityId string, reviewerId *models.UserId,
) error {
	if exec == nil {
		exec = uc.executorFactory.NewExecutor()
	}

	if err := uc.repository.DeleteSanctionCheckMatchWhitelist(ctx, exec, orgId, counterpartyId, entityId, reviewerId); err != nil {
		return err
	}

	return nil
}

func (uc SanctionCheckUsecase) SearchWhitelist(ctx context.Context, exec repositories.Executor,
	orgId string, counterpartyId, entityId *string, reviewerId *models.UserId,
) ([]models.SanctionCheckWhitelist, error) {
	if exec == nil {
		exec = uc.executorFactory.NewExecutor()
	}

	whitelists, err := uc.repository.SearchSanctionCheckMatchWhitelist(ctx, exec, orgId, counterpartyId, entityId)
	if err != nil {
		return nil, err
	}

	return whitelists, nil
}

func (uc SanctionCheckUsecase) MatchAddComment(ctx context.Context, matchId string,
	comment models.SanctionCheckMatchComment,
) (models.SanctionCheckMatchComment, error) {
	if _, err := uc.enforceCanReadOrUpdateSanctionCheckMatch(ctx, matchId); err != nil {
		return models.SanctionCheckMatchComment{}, err
	}

	return uc.repository.AddSanctionCheckMatchComment(ctx, uc.executorFactory.NewExecutor(), comment)
}

func (uc SanctionCheckUsecase) EnrichMatch(ctx context.Context, matchId string) (models.SanctionCheckMatch, error) {
	if _, err := uc.enforceCanReadOrUpdateSanctionCheckMatch(ctx, matchId); err != nil {
		return models.SanctionCheckMatch{}, err
	}

	return uc.EnrichMatchWithoutAuthorization(ctx, matchId)
}

func (uc SanctionCheckUsecase) EnrichMatchWithoutAuthorization(ctx context.Context, matchId string) (models.SanctionCheckMatch, error) {
	match, err := uc.repository.GetSanctionCheckMatch(ctx, uc.executorFactory.NewExecutor(), matchId)
	if err != nil {
		return models.SanctionCheckMatch{}, err
	}

	if match.Enriched {
		return models.SanctionCheckMatch{}, errors.WithDetail(models.UnprocessableEntityError,
			"this sanction check match was already enriched")
	}

	newPayload, err := uc.openSanctionsProvider.EnrichMatch(ctx, match)
	if err != nil {
		return models.SanctionCheckMatch{}, err
	}

	mergedPayload, err := mergePayloads(match.Payload, newPayload)
	if err != nil {
		return models.SanctionCheckMatch{}, errors.Wrap(err,
			"could not merge payloads for match enrichment")
	}

	newMatch, err := uc.repository.UpdateSanctionCheckMatchPayload(ctx,
		uc.executorFactory.NewExecutor(), match, mergedPayload)
	if err != nil {
		return models.SanctionCheckMatch{}, err
	}

	return newMatch, nil
}

func (uc SanctionCheckUsecase) GetEntity(ctx context.Context, entityId string) ([]byte, error) {
	return uc.openSanctionsProvider.EnrichMatch(ctx, models.SanctionCheckMatch{EntityId: entityId})
}

func (uc SanctionCheckUsecase) CreateFiles(ctx context.Context, creds models.Credentials,
	sanctionCheckId string, files []multipart.FileHeader,
) ([]models.SanctionCheckFile, error) {
	sc, err := uc.repository.GetSanctionCheck(ctx, uc.executorFactory.NewExecutor(), sanctionCheckId)
	if err != nil {
		return nil, err
	}

	match, err := uc.enforceCanReadOrUpdateCase(ctx, sc.DecisionId)
	if err != nil {
		return nil, err
	}

	if sc.IsArchived {
		latestSanctionCheck, err := uc.repository.GetActiveSanctionCheckForDecision(ctx,
			uc.executorFactory.NewExecutor(), sc.DecisionId)
		if err != nil {
			return nil, err
		}

		sc = *latestSanctionCheck
	}

	for _, fileHeader := range files {
		if err := validateFileType(fileHeader); err != nil {
			return nil, err
		}
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
		err = writeSanctionCheckFileToBlobStorage(ctx, uc.blobRepository, uc.blobBucketUrl, fileHeader, newFileReference)
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

	uploadedFiles := make([]models.SanctionCheckFile, len(metadata))

	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		for idx, uploadedFile := range metadata {
			file, err := uc.repository.CreateSanctionCheckFile(
				ctx,
				tx,
				models.SanctionCheckFileInput{
					BucketName:      uc.blobBucketUrl,
					SanctionCheckId: sc.Id,
					FileName:        uploadedFile.fileName,
					FileReference:   uploadedFile.fileReference,
				},
			)
			if err != nil {
				return err
			}

			uploadedFiles[idx] = file
		}

		if err := uc.externalRepository.CreateCaseContributor(ctx, tx, match.Case.Id,
			string(creds.ActorIdentity.UserId)); err != nil {
			return errors.Wrap(err, "could not create case contributor for sanction check file upload")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return uploadedFiles, nil
}

func (uc SanctionCheckUsecase) ListFiles(ctx context.Context, sanctionCheckId string) ([]models.SanctionCheckFile, error) {
	sc, err := uc.repository.GetSanctionCheck(ctx, uc.executorFactory.NewExecutor(), sanctionCheckId)
	if err != nil {
		return nil, err
	}

	if _, err := uc.enforceCanReadOrUpdateCase(ctx, sc.DecisionId); err != nil {
		return nil, err
	}

	files, err := uc.repository.ListSanctionCheckFiles(ctx, uc.executorFactory.NewExecutor(), sc.Id)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (uc SanctionCheckUsecase) GetFileDownloadUrl(ctx context.Context, sanctionCheckId, fileId string) (string, error) {
	sc, err := uc.repository.GetSanctionCheck(ctx, uc.executorFactory.NewExecutor(), sanctionCheckId)
	if err != nil {
		return "", err
	}

	if _, err := uc.enforceCanReadOrUpdateCase(ctx, sc.DecisionId); err != nil {
		return "", err
	}

	file, err := uc.repository.GetSanctionCheckFile(ctx, uc.executorFactory.NewExecutor(), sc.Id, fileId)
	if err != nil {
		return "", err
	}

	return uc.blobRepository.GenerateSignedUrl(ctx, uc.blobBucketUrl, file.FileReference)
}

func writeSanctionCheckFileToBlobStorage(ctx context.Context,
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

// Helper functions for enforcing permissions on sanction check actions go below

func (uc SanctionCheckUsecase) enforceCanReadOrUpdateCase(ctx context.Context, decisionId string) (models.Decision, error) {
	creds, _ := utils.CredentialsFromCtx(ctx)

	exec := uc.executorFactory.NewExecutor()
	decision, err := uc.externalRepository.DecisionsById(ctx, exec, []string{decisionId})
	if err != nil {
		return models.Decision{}, err
	}
	if len(decision) == 0 {
		return models.Decision{}, errors.Wrap(models.NotFoundError,
			"could not find the decision linked to the sanction check")
	}

	if creds.Role != models.API_CLIENT {
		if decision[0].Case == nil {
			return decision[0], errors.Wrap(models.UnprocessableEntityError,
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
	}

	return decision[0], nil
}

func (uc SanctionCheckUsecase) enforceCanRefineSanctionCheck(
	ctx context.Context,
	decisionId string,
) (models.Decision, models.SanctionCheckWithMatches, error) {
	sanctionCheck, err := uc.repository.GetActiveSanctionCheckForDecision(ctx,
		uc.executorFactory.NewExecutor(), decisionId)
	if err != nil {
		return models.Decision{}, models.SanctionCheckWithMatches{},
			errors.WithDetail(err, "sanction check does not exist")
	}
	if sanctionCheck == nil {
		return models.Decision{}, models.SanctionCheckWithMatches{},
			errors.WithDetail(models.NotFoundError, "no active sanction check found for this decision")
	}

	if !sanctionCheck.Status.IsRefinable() {
		return models.Decision{}, *sanctionCheck,
			errors.WithDetail(models.NotFoundError,
				"this sanction is not pending review or error")
	}

	decision, err := uc.enforceCanReadOrUpdateCase(ctx, sanctionCheck.DecisionId)
	if err != nil {
		return models.Decision{}, models.SanctionCheckWithMatches{}, err
	}

	return decision, *sanctionCheck, nil
}

type sanctionCheckContextData struct {
	decision models.Decision
	sanction models.SanctionCheck
	match    models.SanctionCheckMatch
}

func (uc SanctionCheckUsecase) enforceCanReadOrUpdateSanctionCheckMatch(
	ctx context.Context,
	matchId string,
) (sanctionCheckContextData, error) {
	match, err := uc.repository.GetSanctionCheckMatch(ctx, uc.executorFactory.NewExecutor(), matchId)
	if err != nil {
		return sanctionCheckContextData{}, err
	}

	sanctionCheck, err := uc.repository.GetSanctionCheckWithoutMatches(ctx,
		uc.executorFactory.NewExecutor(), match.SanctionCheckId)
	if err != nil {
		return sanctionCheckContextData{}, err
	}

	if sanctionCheck.IsArchived {
		return sanctionCheckContextData{}, errors.WithDetail(models.UnprocessableEntityError,
			"sanction check was refined and cannot be reviewed")
	}

	dec, err := uc.enforceCanReadOrUpdateCase(ctx, sanctionCheck.DecisionId)
	if err != nil {
		return sanctionCheckContextData{}, err
	}

	return sanctionCheckContextData{
		decision: dec,
		sanction: sanctionCheck,
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
