package continuous_screening

import (
	"context"
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
// 5. Based on the decision:
//   - If confirmed_hit: marks all other pending matches as "skipped" and the screening as "confirmed_hit"
//   - If no_hit and it's the last pending match: marks the screening as "no_hit"
//   - If no_hit and whitelist flag is set: adds the match to the whitelist
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

		if err := uc.caseEditor.PerformCaseActionSideEffects(ctx, tx, caseData); err != nil {
			return err
		}

		// If the match is confirmed, all other pending matches should be set to "skipped" and the screening to "confirmed_hit"
		if update.Status == models.ScreeningMatchStatusConfirmedHit {
			pendingMatchesIds := pure_utils.Map(pendingMatchesExcludingThis, func(
				m models.ContinuousScreeningMatch,
			) uuid.UUID {
				return m.Id
			})
			_, err = uc.repository.UpdateContinuousScreeningMatchStatusByBatch(
				ctx,
				tx,
				pendingMatchesIds,
				models.ScreeningMatchStatusSkipped,
				reviewerUuid,
			)
			if err != nil {
				return err
			}

			// No huge fan of doing like this because we don't update the continuousScreeningWithMatches object
			// But fine because we don't use it afterwards
			// We should use the result to update the object
			_, err = uc.repository.UpdateContinuousScreeningStatus(
				ctx,
				tx,
				continuousScreeningWithMatches.Id,
				models.ScreeningStatusConfirmedHit,
			)
			if err != nil {
				return err
			}

			err = uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
				CaseId:       continuousScreeningWithMatches.CaseId.String(),
				UserId:       (*string)(reviewerId),
				EventType:    models.ScreeningReviewed,
				ResourceId:   utils.Ptr(continuousScreeningWithMatches.Id.String()),
				ResourceType: utils.Ptr(models.ContinuousScreeningResourceType),
				NewValue:     utils.Ptr(models.ScreeningMatchStatusConfirmedHit.String()),
			})
			if err != nil {
				return err
			}
		}

		// else, if it is the last match pending and it is not a hit, the screening should be set to "no_hit"
		if !continuousScreeningWithMatches.IsPartial && update.Status ==
			models.ScreeningMatchStatusNoHit && len(pendingMatchesExcludingThis) == 0 {
			// No huge fan of doing like this because we don't update the continuousScreeningWithMatches object
			// Bug fine because we don't use it afterwards
			// We should use the result to update the object
			_, err = uc.repository.UpdateContinuousScreeningStatus(
				ctx,
				tx,
				continuousScreeningWithMatches.Id,
				models.ScreeningStatusNoHit,
			)
			if err != nil {
				return err
			}

			err = uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
				CaseId:       continuousScreeningWithMatches.CaseId.String(),
				UserId:       (*string)(reviewerId),
				EventType:    models.ScreeningReviewed,
				ResourceId:   utils.Ptr(continuousScreeningWithMatches.Id.String()),
				ResourceType: utils.Ptr(models.ContinuousScreeningResourceType),
				NewValue:     utils.Ptr(models.ScreeningMatchStatusNoHit.String()),
			})
			if err != nil {
				return err
			}
		}

		if update.Status == models.ScreeningMatchStatusNoHit && update.Whitelist {
			if err := uc.createWhitelist(
				ctx,
				tx,
				continuousScreeningWithMatches.OrgId.String(),
				typedObjectId(
					continuousScreeningWithMatches.ObjectType,
					continuousScreeningWithMatches.ObjectId,
				),
				continuousScreeningMatch.OpenSanctionEntityId,
				reviewerId,
			); err != nil {
				return errors.Wrap(err, "could not whitelist match")
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
		continuousScreening.OrgId.String(),
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
	orgId, counterpartyId, entityId string,
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
		matchIdsToUpdate := pure_utils.Map(matchesToUpdate, func(
			m models.ContinuousScreeningMatch,
		) uuid.UUID {
			return m.Id
		})

		// Update the match statuses
		_, err = uc.repository.UpdateContinuousScreeningMatchStatusByBatch(
			ctx,
			tx,
			matchIdsToUpdate,
			models.ScreeningMatchStatusSkipped,
			reviewerUuid,
		)
		if err != nil {
			return err
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

		clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, continuousScreening.OrgId.String())
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

		// Have the data model table and mapping
		table, mapping, err := uc.GetDataModelTableAndMapping(ctx, tx, config, continuousScreening.ObjectType)
		if err != nil {
			return err
		}

		// Fetch the ingested Data
		ingestedObject, _, err := uc.GetIngestedObject(ctx, clientDbExec, table, continuousScreening.ObjectId)
		if err != nil {
			return err
		}

		// Override configuration max candidates to MAX_SCREENING_CANDIDATES
		config.MatchLimit = MaxScreeningCandidates

		// Do the screening
		screeningWithMatches, err := uc.DoScreening(
			ctx,
			tx,
			ingestedObject,
			mapping,
			config,
			continuousScreening.ObjectType,
			continuousScreening.ObjectId,
		)
		if err != nil {
			return err
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

		return nil
	})
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	return continuousScreening, nil
}
