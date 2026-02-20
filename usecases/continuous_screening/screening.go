package continuous_screening

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// cf: https://api.opensanctions.org/#tag/Matching/operation/match_match__dataset__post
const MaxScreeningCandidates = 500

// UpdateContinuousScreeningMatchStatus updates the status of a continuous screening match
// (e.g., marking it as confirmed_hit or no_hit) and triggers related actions.
//
// General flow:
// 1. Validates the input match ID, reviewer ID, and status (must be confirmed_hit or no_hit). Check if the screening is in case and in review.
// 2. Checks permissions: requires WriteContinuousScreeningHit permission and access to the associated case
// 3. Updates the match status in a transaction
// 4. Performs case action side effects (e.g., updating case status)
// 5. Based on the decision and screening type:
//   - If confirmed_hit on object-triggered screening: immediately marks screening as "confirmed_hit" and skips all other pending matches
//   - If confirmed_hit on dataset-triggered screening: only updates screening status when it's the last pending match
//   - If no_hit and it's the last pending match on object-triggered screening: marks the screening as "no_hit"
//   - If no_hit and it's the last pending match on dataset-triggered screening: marks screening as "confirmed_hit" if any match was confirmed, otherwise "no_hit"
//   - If no_hit (always): adds the match to the whitelist
//
// 6. Creates case events to record the screening review action
func (uc *ContinuousScreeningUsecase) UpdateContinuousScreeningMatchStatus(
	ctx context.Context,
	update models.ScreeningMatchUpdate,
) (models.ContinuousScreeningMatch, error) {
	// Prepare variable used in the transaction
	var updatedMatch models.ContinuousScreeningMatch
	requestedMatchId, err := uuid.Parse(update.MatchId)
	if err != nil {
		return models.ContinuousScreeningMatch{},
			errors.Wrap(models.BadParameterError, "invalid match id")
	}
	var reviewerUuid *uuid.UUID
	if update.ReviewerId != nil {
		tmpUuid, err := uuid.Parse(string(*update.ReviewerId))
		if err != nil {
			return models.ContinuousScreeningMatch{},
				errors.Wrap(models.BadParameterError, "invalid reviewer id")
		}
		reviewerUuid = &tmpUuid
	}
	reviewerId := update.ReviewerId

	if update.Status != models.ScreeningMatchStatusConfirmedHit &&
		update.Status != models.ScreeningMatchStatusNoHit {
		return models.ContinuousScreeningMatch{}, errors.Wrap(models.BadParameterError,
			"invalid status received for screening match, should be 'confirmed_hit' or 'no_hit'")
	}

	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// Fetch continuous screening match and continuous screening object
		continuousScreeningMatch, err := uc.repository.GetContinuousScreeningMatch(ctx, tx, requestedMatchId)
		if err != nil {
			return err
		}
		continuousScreeningWithMatches, err := uc.repository.GetContinuousScreeningWithMatchesById(
			ctx,
			tx,
			continuousScreeningMatch.ContinuousScreeningId,
		)
		if err != nil {
			return err
		}

		// Check if the continuous screening is active and in case
		if continuousScreeningWithMatches.CaseId == nil {
			return errors.Wrap(models.UnprocessableEntityError, "continuous screening is not in case")
		}
		if continuousScreeningWithMatches.Status != models.ScreeningStatusInReview {
			return errors.Wrap(models.UnprocessableEntityError,
				"continuous screening is not in review")
		}

		// CaseId exists, we checked above
		caseData, err := uc.repository.GetCaseById(
			ctx,
			tx,
			continuousScreeningWithMatches.CaseId.String(),
		)
		if err != nil {
			return err
		}

		// Check permission on case and continuous screening
		err = uc.checkPermissionOnCaseAndContinuousScreening(
			ctx,
			tx,
			caseData,
			continuousScreeningWithMatches,
		)
		if err != nil {
			return err
		}

		pendingMatchesExcludingThis := utils.Filter(continuousScreeningWithMatches.Matches, func(
			m models.ContinuousScreeningMatch,
		) bool {
			return m.Id != requestedMatchId && m.Status == models.ScreeningMatchStatusPending
		})

		updatedMatch, err = uc.repository.UpdateContinuousScreeningMatchStatus(
			ctx,
			tx,
			requestedMatchId,
			update.Status,
			reviewerUuid,
		)
		if err != nil {
			return err
		}

		_, err = uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:         continuousScreeningWithMatches.OrgId,
			CaseId:        continuousScreeningWithMatches.CaseId.String(),
			UserId:        (*string)(reviewerId),
			EventType:     models.ScreeningMatchReviewed,
			ResourceId:    utils.Ptr(continuousScreeningMatch.Id.String()),
			ResourceType:  utils.Ptr(models.ContinuousScreeningMatchResourceType),
			NewValue:      utils.Ptr(update.Status.String()),
			PreviousValue: utils.Ptr(continuousScreeningMatch.Status.String()),
		})
		if err != nil {
			return err
		}

		if err := uc.caseEditor.PerformCaseActionSideEffects(ctx, tx, caseData); err != nil {
			return err
		}

		if update.Status == models.ScreeningMatchStatusConfirmedHit {
			if err := uc.handleConfirmedHit(
				ctx,
				tx,
				continuousScreeningWithMatches,
				updatedMatch,
				pendingMatchesExcludingThis,
				reviewerId,
				reviewerUuid,
			); err != nil {
				return err
			}
		}

		// Handle no_hit: update screening status when all matches are reviewed
		isLastPendingMatch := !continuousScreeningWithMatches.IsPartial && len(pendingMatchesExcludingThis) == 0
		if update.Status == models.ScreeningMatchStatusNoHit && isLastPendingMatch {
			if err := uc.handleNoHitLastMatch(
				ctx,
				tx,
				continuousScreeningWithMatches,
				reviewerId,
			); err != nil {
				return err
			}
		}

		// Handle whitelist creation for no_hit matches
		if update.Status == models.ScreeningMatchStatusNoHit {
			if err := uc.handleWhitelistCreation(
				ctx,
				tx,
				continuousScreeningWithMatches,
				continuousScreeningMatch,
				reviewerId,
			); err != nil {
				return err
			}
		}

		return nil
	})

	return updatedMatch, err
}

// Check if the user has permission to change continuous screening and match status
// Check if the user can access and modify the case
func (uc *ContinuousScreeningUsecase) checkPermissionOnCaseAndContinuousScreening(
	ctx context.Context,
	exec repositories.Executor,
	caseData models.Case,
	continuousScreening models.ContinuousScreeningWithMatches,
) error {
	if err := uc.enforceSecurity.WriteContinuousScreeningHit(continuousScreening.OrgId); err != nil {
		return err
	}

	inboxes, err := uc.inboxReader.ListInboxes(
		ctx,
		exec,
		continuousScreening.OrgId,
		false,
	)
	if err != nil {
		return errors.Wrap(err, "could not retrieve organization inboxes")
	}

	inboxIds := pure_utils.Map(inboxes, func(inbox models.Inbox) uuid.UUID {
		return inbox.Id
	})

	return uc.enforceSecurityCase.ReadOrUpdateCase(caseData.GetMetadata(), inboxIds)
}

func (uc *ContinuousScreeningUsecase) createWhitelist(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	counterpartyId, entityId string,
	reviewerId *models.UserId,
) error {
	if err := uc.enforceSecurityScreening.WriteWhitelist(ctx); err != nil {
		return err
	}

	return uc.repository.AddScreeningMatchWhitelist(ctx, exec, orgId, counterpartyId, entityId, reviewerId)
}

// Dismiss function can only be called if the continuous screening is in review and in case and by an admin user
// - Get the continuous screening with matches by id
// - Check if the user has permission to dismiss the continuous screening
// - Check if the continuous screening is in review and in case
// - Update the match statuses to skipped
// - Update the continuous screening status to no_hit
// Return the continuous screening with matches
func (uc *ContinuousScreeningUsecase) DismissContinuousScreening(ctx context.Context,
	continuousScreeningId uuid.UUID, reviewerId *models.UserId,
) (models.ContinuousScreeningWithMatches, error) {
	var reviewerUuid *uuid.UUID
	if reviewerId != nil {
		tmpUuid, err := uuid.Parse(string(*reviewerId))
		if err != nil {
			return models.ContinuousScreeningWithMatches{},
				errors.Wrap(models.BadParameterError, "invalid reviewer id")
		}
		reviewerUuid = &tmpUuid
	}
	var continuousScreening models.ContinuousScreeningWithMatches

	err := uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		var err error
		continuousScreening, err = uc.repository.GetContinuousScreeningWithMatchesById(ctx, tx, continuousScreeningId)
		if err != nil {
			return err
		}

		if err := uc.enforceSecurity.DismissContinuousScreeningHits(
			continuousScreening.OrgId,
		); err != nil {
			return err
		}

		if continuousScreening.CaseId == nil {
			return errors.Wrap(models.UnprocessableEntityError,
				"continuous screening is not in case, can't dismiss")
		}
		if continuousScreening.Status != models.ScreeningStatusInReview {
			return errors.Wrap(models.UnprocessableEntityError,
				"continuous screening is not in review, can't dismiss")
		}

		matchesToUpdate := utils.Filter(continuousScreening.Matches, func(
			m models.ContinuousScreeningMatch,
		) bool {
			return m.Status == models.ScreeningMatchStatusPending
		})

		if len(matchesToUpdate) != 0 {
			// Update the match statuses
			_, err = uc.repository.UpdateContinuousScreeningMatchStatusByBatch(
				ctx,
				tx,
				pure_utils.Map(
					matchesToUpdate,
					func(m models.ContinuousScreeningMatch) uuid.UUID {
						return m.Id
					}),
				models.ScreeningMatchStatusSkipped,
				reviewerUuid,
			)
			if err != nil {
				return err
			}
			_, err = uc.repository.BatchCreateCaseEvents(
				ctx,
				tx,
				pure_utils.Map(matchesToUpdate, func(match models.ContinuousScreeningMatch) models.CreateCaseEventAttributes {
					return models.CreateCaseEventAttributes{
						OrgId:         continuousScreening.OrgId,
						CaseId:        continuousScreening.CaseId.String(),
						UserId:        (*string)(reviewerId),
						EventType:     models.ScreeningMatchReviewed,
						ResourceId:    utils.Ptr(match.Id.String()),
						ResourceType:  utils.Ptr(models.ContinuousScreeningMatchResourceType),
						NewValue:      utils.Ptr(models.ScreeningMatchStatusSkipped.String()),
						PreviousValue: utils.Ptr(match.Status.String()),
					}
				}),
			)
			if err != nil {
				return err
			}
		}

		// Update the continuous screening status
		_, err = uc.repository.UpdateContinuousScreeningStatus(
			ctx,
			tx,
			continuousScreeningId,
			models.ScreeningStatusNoHit,
		)
		if err != nil {
			return err
		}
		_, err = uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:         continuousScreening.OrgId,
			CaseId:        continuousScreening.CaseId.String(),
			UserId:        (*string)(reviewerId),
			EventType:     models.ScreeningReviewed,
			ResourceId:    utils.Ptr(continuousScreening.Id.String()),
			ResourceType:  utils.Ptr(models.ContinuousScreeningResourceType),
			NewValue:      utils.Ptr(models.ScreeningStatusNoHit.String()),
			PreviousValue: utils.Ptr(continuousScreening.Status.String()),
		})
		if err != nil {
			return err
		}

		// Fetch again to have the latest state
		continuousScreening, err = uc.repository.GetContinuousScreeningWithMatchesById(ctx, tx, continuousScreeningId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	return continuousScreening, nil
}

// LoadMoreContinuousScreeningMatches fetches additional screening matches for a continuous screening that has partial results.
//
// 1. Validates that the continuous screening exists, is in "in_review" status and has partial results
// 2. Verifies permissions: requires WriteContinuousScreeningHit permission
// 3. Fetches the screening configuration and data model mapping for the object type
// 4. Retrieves the ingested data for the monitored object
// 5. Re-runs the screening with MatchLimit set to MaxScreeningCandidates (500)
// 6. Filters out matches that already exist in the screening to avoid duplicates
// 7. Inserts only the new matches into the database
// 8. Updates the screening's IsPartial and NumberOfMatches fields:
//   - IsPartial is set to false if the full result set is now loaded
//   - NumberOfMatches is incremented with the new matches count
//
// 9. Returns the updated ContinuousScreeningWithMatches containing all matches (existing + new)
//
// Returns the updated ContinuousScreeningWithMatches containing all matches (existing + newly loaded)
//
// For now, we ignore the case where the new matches don't contains existing matches.
func (uc *ContinuousScreeningUsecase) LoadMoreContinuousScreeningMatches(
	ctx context.Context,
	continuousScreeningId uuid.UUID,
) (models.ContinuousScreeningWithMatches, error) {
	logger := utils.LoggerFromContext(ctx)
	var continuousScreening models.ContinuousScreeningWithMatches

	err := uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// Fetch the continuous screening with matches
		var err error
		continuousScreening, err = uc.repository.GetContinuousScreeningWithMatchesById(ctx, tx, continuousScreeningId)
		if err != nil {
			return err
		}

		if err := uc.enforceSecurity.WriteContinuousScreeningHit(continuousScreening.OrgId); err != nil {
			return err
		}

		// Check if the continuous screening is in review and is Partial
		if continuousScreening.Status != models.ScreeningStatusInReview {
			return errors.Wrap(
				models.UnprocessableEntityError,
				"continuous screening is not in review, can't load more results",
			)
		}
		if !continuousScreening.IsPartial {
			return errors.Wrap(
				models.UnprocessableEntityError,
				"continuous screening is not partial, can't load more results",
			)
		}

		// Fetch the configuration
		config, err := uc.repository.GetContinuousScreeningConfigByStableId(ctx, tx,
			continuousScreening.ContinuousScreeningConfigStableId)
		if err != nil {
			return err
		}

		// Override configuration max candidates to MAX_SCREENING_CANDIDATES
		config.MatchLimit = MaxScreeningCandidates

		var screeningWithMatches models.ScreeningWithMatches

		// Handle different trigger types
		switch {
		case continuousScreening.IsDatasetTriggered():
			// Dataset update trigger: OpenSanction entity to Marble data
			screeningWithMatches, err = uc.loadMoreDatasetUpdate(ctx, tx, continuousScreening, config)
			if err != nil {
				return err
			}
		case continuousScreening.IsObjectTriggered():
			// Object trigger: Marble data to OpenSanction entities
			screeningWithMatches, err = uc.loadMoreObjectTrigger(ctx, tx, continuousScreening, config)
			if err != nil {
				return err
			}
		default:
			// Should not happen
			return errors.Wrapf(
				models.UnprocessableEntityError,
				"unsupported trigger type: %s",
				continuousScreening.TriggerType.String(),
			)
		}

		// Filter matches to keep only new matches compared to the existing ones
		newMatches := utils.Filter(screeningWithMatches.Matches, func(m models.ScreeningMatch) bool {
			return !slices.ContainsFunc(
				continuousScreening.Matches,
				func(csm models.ContinuousScreeningMatch) bool {
					return csm.OpenSanctionEntityId == m.EntityId
				},
			)
		})

		if len(newMatches) == 0 {
			logger.InfoContext(
				ctx,
				"No new matches found in load more",
				"continuous_screening_id", continuousScreeningId,
			)
		}

		// Insert new matches
		insertedMatches, err := uc.repository.InsertContinuousScreeningMatches(
			ctx,
			tx,
			continuousScreeningId,
			pure_utils.Map(newMatches, func(m models.ScreeningMatch) models.ContinuousScreeningMatch {
				return models.ContinuousScreeningMatch{
					OpenSanctionEntityId: m.EntityId,
					Payload:              m.Payload,
				}
			}),
		)
		if err != nil {
			return err
		}

		// Update the continuous screening fields
		continuousScreening.NumberOfMatches += len(insertedMatches)
		continuousScreening.IsPartial = screeningWithMatches.Partial
		continuousScreening.Matches = append(continuousScreening.Matches, insertedMatches...)

		_, err = uc.repository.UpdateContinuousScreening(
			ctx,
			tx,
			continuousScreeningId,
			models.UpdateContinuousScreeningInput{
				NumberOfMatches: utils.Ptr(continuousScreening.NumberOfMatches),
				IsPartial:       utils.Ptr(continuousScreening.IsPartial),
			},
		)
		if err != nil {
			return err
		}

		// Enqueue enrichment task for newly loaded matches
		if err := uc.taskQueueRepository.EnqueueContinuousScreeningMatchEnrichmentTask(
			ctx,
			tx,
			continuousScreening.OrgId,
			continuousScreeningId,
		); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	return continuousScreening, nil
}

// handleConfirmedHit processes a confirmed hit match by updating screening status
// and skipping all other pending matches
func (uc *ContinuousScreeningUsecase) handleConfirmedHit(
	ctx context.Context,
	tx repositories.Transaction,
	screening models.ContinuousScreeningWithMatches,
	confirmedMatch models.ContinuousScreeningMatch,
	pendingMatches []models.ContinuousScreeningMatch,
	reviewerId *models.UserId,
	reviewerUuid *uuid.UUID,
) error {
	// Add risk tags from the confirmed match to the Marble object
	if err := uc.addRiskTagsFromConfirmedMatch(ctx, tx, screening, confirmedMatch, reviewerId); err != nil {
		return errors.Wrap(err, "failed to add risk tags from confirmed match")
	}

	if screening.IsObjectTriggered() {
		// Object-triggered: immediately mark screening as confirmed_hit
		if err := uc.updateScreeningStatusWithEvent(
			ctx,
			tx,
			screening,
			models.ScreeningStatusConfirmedHit,
			reviewerId,
		); err != nil {
			return err
		}

		// Skip all other pending matches
		return uc.skipPendingMatches(ctx, tx, screening, pendingMatches, reviewerId, reviewerUuid)
	}

	// Dataset-triggered: only update screening when it's the last pending match
	isLastPendingMatch := !screening.IsPartial && len(pendingMatches) == 0
	if isLastPendingMatch {
		return uc.updateScreeningStatusWithEvent(
			ctx,
			tx,
			screening,
			models.ScreeningStatusConfirmedHit,
			reviewerId,
		)
	}

	return nil
}

// handleNoHitLastMatch processes the last no_hit match by determining final screening status
func (uc *ContinuousScreeningUsecase) handleNoHitLastMatch(
	ctx context.Context,
	tx repositories.Transaction,
	screening models.ContinuousScreeningWithMatches,
	reviewerId *models.UserId,
) error {
	if screening.IsObjectTriggered() {
		// Object-triggered: mark as no_hit since all matches were reviewed with no confirmed hits
		return uc.updateScreeningStatusWithEvent(
			ctx,
			tx,
			screening,
			models.ScreeningStatusNoHit,
			reviewerId,
		)
	}

	// Dataset-triggered: check if any match was confirmed as a hit
	hasConfirmedHit := slices.ContainsFunc(
		screening.Matches,
		func(m models.ContinuousScreeningMatch) bool {
			return m.Status == models.ScreeningMatchStatusConfirmedHit
		},
	)

	finalStatus := models.ScreeningStatusNoHit
	if hasConfirmedHit {
		finalStatus = models.ScreeningStatusConfirmedHit
	}

	return uc.updateScreeningStatusWithEvent(
		ctx,
		tx,
		screening,
		finalStatus,
		reviewerId,
	)
}

// updateScreeningStatusWithEvent updates screening status and creates a case event
func (uc *ContinuousScreeningUsecase) updateScreeningStatusWithEvent(
	ctx context.Context,
	tx repositories.Transaction,
	screening models.ContinuousScreeningWithMatches,
	newStatus models.ScreeningStatus,
	reviewerId *models.UserId,
) error {
	_, err := uc.repository.UpdateContinuousScreeningStatus(
		ctx,
		tx,
		screening.Id,
		newStatus,
	)
	if err != nil {
		return err
	}

	_, err = uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
		OrgId:         screening.OrgId,
		CaseId:        screening.CaseId.String(),
		UserId:        (*string)(reviewerId),
		EventType:     models.ScreeningReviewed,
		ResourceId:    utils.Ptr(screening.Id.String()),
		ResourceType:  utils.Ptr(models.ContinuousScreeningResourceType),
		NewValue:      utils.Ptr(newStatus.String()),
		PreviousValue: utils.Ptr(screening.Status.String()),
	})

	return err
}

// skipPendingMatches marks all pending matches as skipped and creates case events
func (uc *ContinuousScreeningUsecase) skipPendingMatches(
	ctx context.Context,
	tx repositories.Transaction,
	screening models.ContinuousScreeningWithMatches,
	pendingMatches []models.ContinuousScreeningMatch,
	reviewerId *models.UserId,
	reviewerUuid *uuid.UUID,
) error {
	if len(pendingMatches) == 0 {
		return nil
	}

	matchIds := pure_utils.Map(pendingMatches, func(m models.ContinuousScreeningMatch) uuid.UUID {
		return m.Id
	})

	_, err := uc.repository.UpdateContinuousScreeningMatchStatusByBatch(
		ctx,
		tx,
		matchIds,
		models.ScreeningMatchStatusSkipped,
		reviewerUuid,
	)
	if err != nil {
		return err
	}

	_, err = uc.repository.BatchCreateCaseEvents(
		ctx,
		tx,
		pure_utils.Map(pendingMatches, func(match models.ContinuousScreeningMatch) models.CreateCaseEventAttributes {
			return models.CreateCaseEventAttributes{
				OrgId:         screening.OrgId,
				CaseId:        screening.CaseId.String(),
				UserId:        (*string)(reviewerId),
				EventType:     models.ScreeningMatchReviewed,
				ResourceId:    utils.Ptr(match.Id.String()),
				ResourceType:  utils.Ptr(models.ContinuousScreeningMatchResourceType),
				NewValue:      utils.Ptr(models.ScreeningMatchStatusSkipped.String()),
				PreviousValue: utils.Ptr(match.Status.String()),
			}
		}),
	)

	return err
}

// handleWhitelistCreation adds a match to the whitelist when marked as no_hit
func (uc *ContinuousScreeningUsecase) handleWhitelistCreation(
	ctx context.Context,
	tx repositories.Transaction,
	screening models.ContinuousScreeningWithMatches,
	match models.ContinuousScreeningMatch,
	reviewerId *models.UserId,
) error {
	var counterpartyId string
	var openSanctionEntityId string

	switch {
	case screening.IsDatasetTriggered():
		if screening.OpenSanctionEntityId == nil {
			return errors.New("OpenSanctionEntityId is missing for DatasetUpdated screening type")
		}

		// The counterparty (Marble entity) is the one being screened and saved in the match as OpenSanctionEntityId
		counterpartyId = match.OpenSanctionEntityId
		openSanctionEntityId = *screening.OpenSanctionEntityId
	case screening.IsObjectTriggered():
		if screening.ObjectType == nil || screening.ObjectId == nil {
			return errors.New("object type or object id is missing for Marble initiated screening")
		}

		counterpartyId = pure_utils.MarbleEntityIdBuilder(
			*screening.ObjectType,
			*screening.ObjectId,
		)
		openSanctionEntityId = match.OpenSanctionEntityId
	default:
		// Should not happen
		return errors.New("unable to determine screening type for whitelist creation")
	}

	if err := uc.createWhitelist(
		ctx,
		tx,
		screening.OrgId,
		counterpartyId,
		openSanctionEntityId,
		reviewerId,
	); err != nil {
		return errors.Wrap(err, "could not whitelist match")
	}

	return nil
}

// addRiskTagsFromConfirmedMatch extracts tags from the entity payload and stores them
// on the Marble object when a screening match is confirmed as a hit.
func (uc *ContinuousScreeningUsecase) addRiskTagsFromConfirmedMatch(
	ctx context.Context,
	tx repositories.Transaction,
	screening models.ContinuousScreeningWithMatches,
	match models.ContinuousScreeningMatch,
	reviewerId *models.UserId,
) error {
	// Determine object type, ID, and entity payload based on trigger type
	var objectType, objectId string
	var openSanctionsEntityId string
	var entityPayload []byte

	switch {
	case screening.IsObjectTriggered():
		// Marble entity → OpenSanctions match
		// Topics come from the MATCH payload (the OpenSanctions entity that matched)
		if screening.ObjectType == nil || screening.ObjectId == nil {
			return errors.New("object type or id missing for object-triggered screening")
		}
		objectType = *screening.ObjectType
		objectId = *screening.ObjectId
		openSanctionsEntityId = match.OpenSanctionEntityId
		entityPayload = match.Payload // Topics from match

	case screening.IsDatasetTriggered():
		// OpenSanctions update → Marble entity match
		// Topics come from the SCREENING's entity payload (the updated OpenSanctions entity)
		if match.Metadata == nil {
			return errors.New("match metadata missing for dataset-triggered screening")
		}
		objectType = match.Metadata.ObjectType
		objectId = match.Metadata.ObjectId
		if screening.OpenSanctionEntityId != nil {
			openSanctionsEntityId = *screening.OpenSanctionEntityId
		}
		entityPayload = screening.OpenSanctionEntityPayload // Topics from screening entity

	default:
		return errors.Errorf("unsupported trigger type: %s", screening.TriggerType)
	}

	// Extract and map tags from the entity payload
	tags, err := models.ExtractRiskTagsFromEntityPayload(entityPayload)
	if err != nil {
		return errors.Wrap(err, "failed to extract risk tags from entity payload")
	}
	if len(tags) == 0 {
		// No relevant tags to add - this is OK, just skip
		return nil
	}

	// Create the upsert input (will APPEND tags, not replace)
	input := models.NewObjectRiskTagFromContinuousScreeningReview(
		screening.OrgId,
		objectType,
		objectId,
		tags,
		screening.Id,
		openSanctionsEntityId,
	)
	input.AnnotatedBy = reviewerId

	return uc.objectRiskTagWriter.AttachObjectRiskTags(ctx, tx, input)
}

// loadMoreObjectTrigger handles load more for object trigger screenings (Marble to OpenSanction direction)
func (uc *ContinuousScreeningUsecase) loadMoreObjectTrigger(
	ctx context.Context,
	exec repositories.Executor,
	continuousScreening models.ContinuousScreeningWithMatches,
	config models.ContinuousScreeningConfig,
) (models.ScreeningWithMatches, error) {
	if continuousScreening.ObjectType == nil || continuousScreening.ObjectId == nil {
		return models.ScreeningWithMatches{}, errors.Wrap(
			models.UnprocessableEntityError,
			"object type or object id is missing for Marble initiated screening",
		)
	}

	clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, continuousScreening.OrgId)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	// Have the data model table and mapping
	table, mapping, err := uc.GetDataModelTableAndMapping(ctx, exec, config, *continuousScreening.ObjectType)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	// Fetch the ingested Data
	ingestedObject, _, err := uc.GetIngestedObject(ctx, clientDbExec, table, *continuousScreening.ObjectId)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	// Do the screening
	return uc.DoScreening(
		ctx,
		exec,
		ingestedObject,
		mapping,
		config,
		*continuousScreening.ObjectType,
		*continuousScreening.ObjectId,
	)
}

// loadMoreDatasetUpdate handles load more for dataset update trigger screenings (OpenSanction to Marble direction)
func (uc *ContinuousScreeningUsecase) loadMoreDatasetUpdate(
	ctx context.Context,
	exec repositories.Executor,
	continuousScreening models.ContinuousScreeningWithMatches,
	config models.ContinuousScreeningConfig,
) (models.ScreeningWithMatches, error) {
	if continuousScreening.OpenSanctionEntityId == nil {
		return models.ScreeningWithMatches{}, errors.Wrap(
			models.UnprocessableEntityError,
			"OpenSanctionEntityId is missing for DatasetUpdated screening type",
		)
	}

	// Parse the OpenSanction entity payload to extract the entity data
	var entity models.OpenSanctionsDeltaFileEntity
	if err := json.Unmarshal(continuousScreening.OpenSanctionEntityPayload, &entity); err != nil {
		return models.ScreeningWithMatches{}, errors.Wrap(err,
			"failed to unmarshal OpenSanction entity payload")
	}

	// Perform the screening using the dedicated method for entity screening
	return uc.DoScreeningForEntity(ctx, exec, entity, config, continuousScreening.OrgId)
}
