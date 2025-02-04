package usecases

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set/v2"
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
	Search(context.Context, models.OpenSanctionsQuery) (models.SanctionCheckWithMatches, error)
}

type SanctionCheckDecisionRepository interface {
	DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error)
}

type SanctionCheckOrganizationRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
}

type SanctionCheckCaseRepository interface {
	CreateCaseEvent(
		ctx context.Context,
		exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes,
	) error
	CreateCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) error
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
	GetSanctionChecksForDecision(ctx context.Context, exec repositories.Executor, decisionId string) (
		[]models.SanctionCheckWithMatches, error)
	GetSanctionCheck(context.Context, repositories.Executor, string) (models.SanctionCheckWithMatches, error)
	GetSanctionCheckWithoutMatches(context.Context, repositories.Executor, string) (models.SanctionCheck, error)
	ArchiveSanctionCheck(context.Context, repositories.Executor, string) error
	InsertSanctionCheck(context.Context, repositories.Executor, string,
		models.SanctionCheckWithMatches) (models.SanctionCheckWithMatches, error)
	UpdateSanctionCheckStatus(ctx context.Context, exec repositories.Executor, id string,
		status models.SanctionCheckStatus) error

	ListSanctionCheckMatches(ctx context.Context, exec repositories.Executor, sanctionCheckId string, forUpdate ...bool) (
		[]models.SanctionCheckMatch, error)
	GetSanctionCheckMatch(ctx context.Context, exec repositories.Executor, matchId string) (models.SanctionCheckMatch, error)
	UpdateSanctionCheckMatchStatus(ctx context.Context, exec repositories.Executor,
		match models.SanctionCheckMatch, update models.SanctionCheckMatchUpdate) (models.SanctionCheckMatch, error)
	ListSanctionCheckCommentsByIds(ctx context.Context, exec repositories.Executor, ids []string) (
		[]models.SanctionCheckMatchComment, error)
	AddSanctionCheckMatchComment(ctx context.Context, exec repositories.Executor,
		comment models.SanctionCheckMatchComment) (models.SanctionCheckMatchComment, error)
	ListSanctionCheckMatchComments(ctx context.Context, exec repositories.Executor, matchId string) (
		[]models.SanctionCheckMatchComment, error)
	CreateSanctionCheckFile(ctx context.Context, exec repositories.Executor,
		input models.SanctionCheckFileInput) (models.SanctionCheckFile, error)
	GetSanctionCheckFile(ctx context.Context, exec repositories.Executor, matchId, fileId string) (models.SanctionCheckFile, error)
	ListSanctionCheckFiles(ctx context.Context, exec repositories.Executor, matchId string) ([]models.SanctionCheckFile, error)
}

type SanctionCheckUsecase struct {
	enforceSecurityDecision SanctionCheckEnforceSecurityDecision
	enforceSecurityCase     SanctionCheckEnforceSecurityCase

	caseRepository                SanctionCheckCaseRepository
	organizationRepository        SanctionCheckOrganizationRepository
	decisionRepository            SanctionCheckDecisionRepository
	inboxReader                   SanctionCheckInboxReader
	openSanctionsProvider         SanctionCheckProvider
	sanctionCheckConfigRepository SanctionCheckConfigRepository
	blobBucketUrl                 string
	blobRepository                repositories.BlobRepository
	repository                    SanctionCheckRepository
	executorFactory               executor_factory.ExecutorFactory
	transactionFactory            executor_factory.TransactionFactory
}

func (uc SanctionCheckUsecase) CheckDataset(ctx context.Context) (models.OpenSanctionsDataset, error) {
	return uc.openSanctionsProvider.GetLatestLocalDataset(ctx)
}

func (uc SanctionCheckUsecase) ListSanctionChecks(ctx context.Context, decisionId string) ([]models.SanctionCheckWithMatches, error) {
	exec := uc.executorFactory.NewExecutor()
	decisions, err := uc.decisionRepository.DecisionsById(ctx, exec, []string{decisionId})
	if err != nil {
		return nil, err
	}
	if len(decisions) == 0 {
		return nil, errors.Wrap(models.NotFoundError, "requested decision does not exist")
	}

	if _, err = uc.enforceCanReadOrUpdateCase(ctx, decisions[0].DecisionId); err != nil {
		return nil, err
	}

	scs, err := uc.repository.GetSanctionChecksForDecision(ctx, exec, decisions[0].DecisionId)
	if err != nil {
		return nil, err
	}

	matchIds := set.New[string](0)
	matchIdToMatch := make(map[string]*models.SanctionCheckMatch)

	for sidx, sc := range scs {
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

	matches.Datasets = query.Config.Datasets
	matches.OrgConfig = org.OpenSanctionsConfig

	return matches, err
}

func (uc SanctionCheckUsecase) Refine(ctx context.Context, refine models.SanctionCheckRefineRequest,
	requestedBy models.UserId,
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
		OrgConfig: sc.OrgConfig,
		Config:    *scc,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"macron"},
		},
	}

	sanctionCheck, err := uc.Execute(ctx, decision.OrganizationId, query)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	sanctionCheck.IsManual = true
	sanctionCheck.RequestedBy = utils.Ptr(string(requestedBy))

	sanctionCheck, err = executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (models.SanctionCheckWithMatches, error) {
		if err := uc.repository.ArchiveSanctionCheck(ctx, tx, decision.DecisionId); err != nil {
			return models.SanctionCheckWithMatches{}, err
		}

		if sanctionCheck, err = uc.repository.InsertSanctionCheck(ctx, tx,
			decision.DecisionId, sanctionCheck); err != nil {
			return models.SanctionCheckWithMatches{}, err
		}

		return sanctionCheck, err
	})
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	return sanctionCheck, nil
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
		return data.match, errors.Wrap(models.BadParameterError, "this sanction is not pending review")
	}

	if data.match.Status != models.SanctionMatchStatusPending {
		return data.match, errors.Wrap(models.BadParameterError, "this match is not pending review")
	}

	var updatedMatch models.SanctionCheckMatch
	err = uc.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			allMatches, err := uc.repository.ListSanctionCheckMatches(ctx, tx, data.sanction.Id, true)
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

					err = uc.caseRepository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
						CaseId:       data.decision.Case.Id,
						UserId:       string(update.ReviewerId),
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

				err = uc.caseRepository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
					CaseId:       data.decision.Case.Id,
					UserId:       string(update.ReviewerId),
					EventType:    models.SanctionCheckReviewed,
					ResourceId:   &data.decision.DecisionId,
					ResourceType: utils.Ptr(models.DecisionResourceType),
					NewValue:     utils.Ptr(models.SanctionMatchStatusNoHit.String()),
				})
				if err != nil {
					return err
				}
			}
			return nil
		},
	)

	return updatedMatch, err
}

func (uc SanctionCheckUsecase) MatchListComments(ctx context.Context, matchId string) ([]models.SanctionCheckMatchComment, error) {
	if _, err := uc.enforceCanReadOrUpdateSanctionCheckMatch(ctx, matchId); err != nil {
		return nil, err
	}

	return uc.repository.ListSanctionCheckMatchComments(ctx, uc.executorFactory.NewExecutor(), matchId)
}

func (uc SanctionCheckUsecase) MatchAddComment(ctx context.Context, matchId string,
	comment models.SanctionCheckMatchComment,
) (models.SanctionCheckMatchComment, error) {
	if _, err := uc.enforceCanReadOrUpdateSanctionCheckMatch(ctx, matchId); err != nil {
		return models.SanctionCheckMatchComment{}, err
	}

	return uc.repository.AddSanctionCheckMatchComment(ctx, uc.executorFactory.NewExecutor(), comment)
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
					SanctionCheckId: sanctionCheckId,
					FileName:        uploadedFile.fileName,
					FileReference:   uploadedFile.fileReference,
				},
			)
			if err != nil {
				return err
			}

			uploadedFiles[idx] = file
		}

		if err := uc.caseRepository.CreateCaseContributor(ctx, tx, match.Case.Id,
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

func (uc SanctionCheckUsecase) enforceCanRefineSanctionCheck(
	ctx context.Context,
	decisionId string,
) (models.Decision, models.SanctionCheckWithMatches, error) {
	sanctionCheck, err := uc.repository.GetActiveSanctionCheckForDecision(ctx,
		uc.executorFactory.NewExecutor(), decisionId)
	if err != nil {
		return models.Decision{}, models.SanctionCheckWithMatches{},
			errors.Wrap(err, "sanction check does not exist")
	}
	if sanctionCheck == nil {
		return models.Decision{}, models.SanctionCheckWithMatches{},
			errors.Wrap(models.NotFoundError, "no active sanction check found for this decision")
	}

	if !sanctionCheck.Status.IsReviewable() {
		return models.Decision{}, *sanctionCheck,
			errors.Wrap(models.NotFoundError, "this sanction is not pending review")
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
