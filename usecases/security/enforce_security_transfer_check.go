package security

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (e *EnforceSecurityImpl) CreateTransfer(ctx context.Context, organizationId string, partnerId string) error {
	return errors.Join(
		e.Permission(models.TRANSFER_CREATE),
		utils.EnforceOrganizationAccess(e.Credentials, organizationId),
		utils.EnforcePartnerAccess(e.Credentials, partnerId),
	)
}

func (e *EnforceSecurityImpl) ReadTransfer(ctx context.Context, transferMapping models.TransferMapping) error {
	return errors.Join(
		e.Permission(models.TRANSFER_READ),
		utils.EnforceOrganizationAccess(e.Credentials, transferMapping.OrganizationId),
		utils.EnforcePartnerAccess(e.Credentials, transferMapping.PartnerId),
	)
}

func (e *EnforceSecurityImpl) UpdateTransfer(ctx context.Context, transferMapping models.TransferMapping) error {
	return errors.Join(
		e.Permission(models.TRANSFER_UPDATE),
		utils.EnforceOrganizationAccess(e.Credentials, transferMapping.OrganizationId),
		utils.EnforcePartnerAccess(e.Credentials, transferMapping.PartnerId),
	)
}

func (e *EnforceSecurityImpl) ReadTransferData(ctx context.Context, partnerId string) error {
	return errors.Join(
		e.Permission(models.TRANSFER_READ),
		utils.EnforcePartnerAccess(e.Credentials, partnerId),
	)
}

func (e *EnforceSecurityImpl) ReadTransferAlert(
	ctx context.Context,
	transferAlert models.TransferAlert,
	senderOrBeneficiary string,
) error {
	var err error

	switch senderOrBeneficiary {
	case "sender":
		err = utils.EnforcePartnerAccess(e.Credentials, transferAlert.SenderPartnerId)
	case "beneficiary":
		err = utils.EnforcePartnerAccess(e.Credentials, transferAlert.BeneficiaryPartnerId)
	default:
		err = errors.Newf("invalid access type %s", senderOrBeneficiary)
	}

	return errors.Join(
		err,
		e.Permission(models.TRANSFER_ALERT_READ),
		utils.EnforceOrganizationAccess(e.Credentials, transferAlert.OrganizationId),
	)
}

func (e *EnforceSecurityImpl) UpdateTransferAlert(
	ctx context.Context,
	transferAlert models.TransferAlert,
	senderOrBeneficiary string,
) error {
	var err error

	switch senderOrBeneficiary {
	case "sender":
		err = utils.EnforcePartnerAccess(e.Credentials, transferAlert.SenderPartnerId)
	case "beneficiary":
		err = utils.EnforcePartnerAccess(e.Credentials, transferAlert.BeneficiaryPartnerId)
	default:
		err = errors.Newf("invalid access type %s", senderOrBeneficiary)
	}

	return errors.Join(
		err,
		e.Permission(models.TRANSFER_ALERT_UPDATE),
		utils.EnforceOrganizationAccess(e.Credentials, transferAlert.OrganizationId),
	)
}

func (e *EnforceSecurityImpl) CreateTransferAlert(ctx context.Context, organizationId string, partnerId string) error {
	return errors.Join(
		e.Permission(models.TRANSFER_ALERT_CREATE),
		utils.EnforceOrganizationAccess(e.Credentials, organizationId),
		utils.EnforcePartnerAccess(e.Credentials, partnerId),
	)
}
