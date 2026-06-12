package usecases

import (
	"context"
	"fmt"
	"net/mail"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"

	"github.com/cockroachdb/errors"
)

type SeedUseCase struct {
	transactionFactory     executor_factory.TransactionFactory
	executorFactory        executor_factory.ExecutorFactory
	userRepository         repositories.UserRepository
	organizationCreator    organization.OrganizationCreator
	organizationRepository repositories.OrganizationRepository
	customListRepository   repositories.CustomListRepository
	firebaseAdmin          idp.Adminer
}

func (usecase *SeedUseCase) SeedMarbleAdmins(ctx context.Context, firstMarbleAdminEmail string) error {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	_, err := usecase.userRepository.CreateUser(ctx, exec, models.CreateUser{
		Email: firstMarbleAdminEmail,
		Role:  models.MARBLE_ADMIN,
	})

	// ignore user already added
	if err != nil && !repositories.IsUniqueViolationError(err) {
		return err
	}
	logger.InfoContext(ctx, fmt.Sprintf("Created marble admin user with email %s (or already exists)", firstMarbleAdminEmail))

	// Mirror the API user-creation path and create the user in Firebase too. This is
	// idempotent (the Firebase client skips creation when the user already exists), so it
	// is safe to run on every startup and heals users that exist in DB but not in Firebase.
	if err := usecase.createFirebaseUser(ctx, firstMarbleAdminEmail); err != nil {
		return err
	}
	return nil
}

// createFirebaseUser creates the user in Firebase if a Firebase admin client is configured.
// It is a no-op when Firebase is not configured, and idempotent otherwise.
func (usecase *SeedUseCase) createFirebaseUser(ctx context.Context, email string) error {
	if usecase.firebaseAdmin == nil {
		return nil
	}
	// Seeded users have no first/last name, so use the email as the Firebase display name
	// (the Firebase client rejects an empty display name).
	if err := usecase.firebaseAdmin.CreateUser(ctx, email, email); err != nil {
		return errors.Wrap(err, "could not create Firebase user")
	}
	return nil
}

// This method is supposed to be used as a script when starting the server, not from the API
// Hence it does not enforce any authorization, since there is also no user credentials context
func (usecase *SeedUseCase) CreateOrgAndUser(ctx context.Context, input models.InitOrgInput) error {
	if input.OrgName == "" {
		return errors.New("Cannot create organization or org admin with empty name in CreateOrgAndUser")
	}
	if input.AdminEmail != "" {
		_, err := mail.ParseAddress(input.AdminEmail)
		if err != nil {
			return errors.New(fmt.Sprintf("Invalid email address %s in CreateOrgAndUser", input.AdminEmail))
		}
	}
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	var targetOrg models.Organization
	allOrgs, err := usecase.organizationRepository.AllOrganizations(ctx, exec)
	if err != nil {
		return err
	}
	for _, org := range allOrgs {
		if org.Name == input.OrgName {
			targetOrg = org
			logger.InfoContext(
				ctx,
				fmt.Sprintf("Organization %s already exists for name %s", targetOrg.Id, input.OrgName),
			)
			break
		}
	}

	if targetOrg.Id == uuid.Nil {
		targetOrg, err = usecase.organizationCreator.CreateOrganization(ctx, models.CreateOrganizationInput{
			Name: input.OrgName,
		})
		if err != nil && !repositories.IsUniqueViolationError(err) {
			return err
		}
		logger.InfoContext(
			ctx,
			fmt.Sprintf("Created organization %s with name %s", targetOrg.Id, input.OrgName),
		)
	}

	if input.AdminEmail != "" {
		_, err := usecase.userRepository.CreateUser(ctx, exec, models.CreateUser{
			Email:          input.AdminEmail,
			OrganizationId: targetOrg.Id,
			Role:           models.ADMIN,
		})
		if err != nil && !repositories.IsUniqueViolationError(err) {
			return err
		}
		logger.InfoContext(
			ctx,
			fmt.Sprintf("Created admin user for organization %s with email %s (or already exists)", targetOrg.Id, input.AdminEmail),
		)

		if err := usecase.createFirebaseUser(ctx, input.AdminEmail); err != nil {
			return err
		}
	}

	return nil
}
