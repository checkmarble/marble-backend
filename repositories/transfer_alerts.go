package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func selectTransferAlerts() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectTransferAlertsColumn...).
		From(dbmodels.TABLE_TRANSFER_ALERTS)
}

func (repo *MarbleDbRepository) GetTransferAlert(ctx context.Context, exec Executor, id string) (models.TransferAlert, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.TransferAlert{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectTransferAlerts().Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptTransferAlert,
	)
}

func (repo *MarbleDbRepository) ListTransferAlerts(
	ctx context.Context,
	exec Executor,
	organizationId string,
	partnerId string,
	senderOrBeneficiary string,
) ([]models.TransferAlert, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	var partnerFilterField string
	switch senderOrBeneficiary {
	case "sender":
		partnerFilterField = "sender_partner_id"
	case "beneficiary":
		partnerFilterField = "beneficiary_partner_id"
	default:
		return nil, errors.Newf(`invalid value for senderOrBeneficiary "%s" in MarbleDbRepository.ListTransferAlerts`, senderOrBeneficiary)
	}

	return SqlToListOfModels(
		ctx,
		exec,
		selectTransferAlerts().
			Where(squirrel.Eq{"organization_id": organizationId}).
			Where(squirrel.Eq{partnerFilterField: partnerId}).
			OrderBy("created_at DESC"),
		dbmodels.AdaptTransferAlert,
	)
}

func (repo *MarbleDbRepository) CreateTransferAlert(
	ctx context.Context,
	exec Executor,
	TransferAlert models.TransferAlert,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_TRANSFER_ALERTS).
			Columns(
				"id",
				"transfer_id",
				"organization_id",
				"sender_partner_id",
				"beneficiary_partner_id",
				"created_at",
				"status",
				"message",
				"transfer_end_to_end_id",
				"beneficiary_iban",
				"sender_iban",
			).
			Values(
				TransferAlert.Id,
				TransferAlert.TransferId,
				TransferAlert.OrganizationId,
				TransferAlert.SenderPartnerId,
				TransferAlert.BeneficiaryPartnerId,
				TransferAlert.CreatedAt,
				TransferAlert.Status,
				TransferAlert.Message,
				TransferAlert.TransferEndToEndId,
				TransferAlert.BeneficiaryIban,
				TransferAlert.SenderIban,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateTransferAlertAsBeneficiary(
	ctx context.Context,
	exec Executor,
	alertId string,
	input models.TransferAlertUpdateBodyBeneficiary,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_TRANSFER_ALERTS).
		Where(squirrel.Eq{"id": alertId})

	if input.Status.Valid {
		query = query.Set("status", input.Status.String)
		err := ExecBuilder(ctx, exec, query)
		return err
	}

	return nil
}

func (repo *MarbleDbRepository) UpdateTransferAlertAsSender(
	ctx context.Context,
	exec Executor,
	alertId string,
	input models.TransferAlertUpdateBodySender,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_TRANSFER_ALERTS).
		Where(squirrel.Eq{"id": alertId})

	n := 0
	if input.Message.Valid {
		query = query.Set("message", input.Message.String)
		n += 1
	}
	if input.BeneficiaryIban.Valid {
		query = query.Set("beneficiary_iban", input.BeneficiaryIban.String)
		n += 1
	}
	if input.SenderIban.Valid {
		query = query.Set("sender_iban", input.SenderIban.String)
		n += 1
	}
	if input.TransferEndToEndId.Valid {
		query = query.Set("transfer_end_to_end_id", input.TransferEndToEndId.String)
		n += 1
	}

	if n == 0 {
		return nil
	}
	return ExecBuilder(ctx, exec, query)
}
