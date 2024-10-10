package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type protectedLicenseRepository interface {
	GetLicenseById(ctx context.Context, exec repositories.Executor, licenseId string) (models.License, error)
	ListLicenses(ctx context.Context, exec repositories.Executor) ([]models.License, error)
	CreateLicense(ctx context.Context, exec repositories.Executor, license models.License) error
	UpdateLicense(ctx context.Context, exec repositories.Executor,
		updateLicenseInput models.UpdateLicenseInput) error
}

type enforceSecurityLicense interface {
	ListLicenses(ctx context.Context) error
	CreateLicense(ctx context.Context) error
	UpdateLicense(ctx context.Context) error
}

type ProtectedLicenseUseCase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	enforceSecurity    enforceSecurityLicense
	licenseRepository  protectedLicenseRepository
}

func (usecase *ProtectedLicenseUseCase) ListLicenses(ctx context.Context) ([]models.License, error) {
	if err := usecase.enforceSecurity.ListLicenses(ctx); err != nil {
		return []models.License{}, err
	}

	exec := usecase.executorFactory.NewExecutor()
	licenses, err := usecase.licenseRepository.ListLicenses(ctx, exec)
	if err != nil {
		return []models.License{}, err
	}
	return licenses, nil
}

func (usecase *ProtectedLicenseUseCase) GetLicenseById(ctx context.Context, licenseId string) (models.License, error) {
	if err := usecase.enforceSecurity.ListLicenses(ctx); err != nil {
		return models.License{}, err
	}

	exec := usecase.executorFactory.NewExecutor()
	license, err := usecase.licenseRepository.GetLicenseById(ctx, exec, licenseId)
	if err != nil {
		return models.License{}, err
	}
	return license, nil
}

func (usecase *ProtectedLicenseUseCase) CreateLicense(ctx context.Context, input models.CreateLicenseInput) (models.License, error) {
	if err := usecase.enforceSecurity.CreateLicense(ctx); err != nil {
		return models.License{}, err
	}
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.License, error) {
		licenseId := uuid.NewString()
		err := usecase.licenseRepository.CreateLicense(
			ctx,
			tx,
			models.License{
				Id:                  licenseId,
				Key:                 generateLicense(),
				ExpirationDate:      input.ExpirationDate,
				OrganizationName:    input.OrganizationName,
				Description:         input.Description,
				LicenseEntitlements: input.LicenseEntitlements,
			},
		)
		if err != nil {
			return models.License{}, err
		}
		return usecase.licenseRepository.GetLicenseById(ctx, tx, licenseId)
	})
}

func generateLicense() string {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		panic(fmt.Errorf("generateLicense: %w", err))
	}
	return hex.EncodeToString(key)
}

func (usecase *ProtectedLicenseUseCase) UpdateLicense(ctx context.Context, input models.UpdateLicenseInput) (models.License, error) {
	if err := usecase.enforceSecurity.UpdateLicense(ctx); err != nil {
		return models.License{}, err
	}

	if err := input.Validate(); err != nil {
		return models.License{}, err
	}

	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.License, error) {
		err := usecase.licenseRepository.UpdateLicense(ctx, tx, input)
		if err != nil {
			return models.License{}, err
		}
		return usecase.licenseRepository.GetLicenseById(ctx, tx, input.Id)
	})
}

type publicLicenseRepository interface {
	GetLicenseByKey(ctx context.Context, exec repositories.Executor, licenseKey string) (models.License, error)
}

type PublicLicenseUseCase struct {
	executorFactory   executor_factory.ExecutorFactory
	licenseRepository publicLicenseRepository
}

func (usecase *PublicLicenseUseCase) ValidateLicense(ctx context.Context, licenseKey string) (models.LicenseValidation, error) {
	exec := usecase.executorFactory.NewExecutor()
	license, err := usecase.licenseRepository.GetLicenseByKey(ctx, exec, licenseKey)
	if err != nil {
		if !errors.Is(err, models.NotFoundError) {
			utils.LogAndReportSentryError(ctx, err)
		}

		return models.LicenseValidation{
			LicenseValidationCode: models.NOT_FOUND,
		}, nil
	}
	if license.SuspendedAt.Valid {
		return models.LicenseValidation{
			LicenseValidationCode: models.SUSPENDED,
		}, nil
	}
	if license.ExpirationDate.Before(time.Now()) {
		return models.LicenseValidation{
			LicenseValidationCode: models.EXPIRED,
		}, nil
	}

	return models.LicenseValidation{
		LicenseValidationCode: models.VALID,
		LicenseEntitlements:   license.LicenseEntitlements,
	}, nil
}
