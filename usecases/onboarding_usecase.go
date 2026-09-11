package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
)

const initialOnboardingLockKey = "marble_initial_onboarding"

type OnboardingUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	orgRepository      repositories.OrganizationRepository
	userRepository     repositories.UserRepository
	grantRepository    repositories.GrantRepository
	tokenProvider      auth.TokenProvider
	firebase           idp.Adminer
}

func NewOnboardingUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	orgRepository repositories.OrganizationRepository,
	userRepository repositories.UserRepository,
	grantRepository repositories.GrantRepository,
	tokenProvider auth.TokenProvider,
	firebase idp.Adminer,
) OnboardingUsecase {
	return OnboardingUsecase{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		orgRepository:      orgRepository,
		userRepository:     userRepository,
		grantRepository:    grantRepository,
		tokenProvider:      tokenProvider,
		firebase:           firebase,
	}
}

func (uc OnboardingUsecase) CreateInitialOrganization(ctx context.Context, req dto.CreateInitialOrg) error {
	usesFirebase := uc.tokenProvider == auth.TokenProviderFirebase

	if usesFirebase {
		if req.Password == "" {
			return errors.Wrap(models.BadParameterError,
				"a password is required when configured to use Firebase authentication")
		}
		if uc.firebase == nil {
			return errors.New("configured to use Firebase authentication, but no Firebase admin client is available")
		}
	}

	hasAnOrganization, err := uc.orgRepository.HasOrganizations(ctx, uc.executorFactory.NewExecutor())
	if err != nil {
		return err
	}
	if hasAnOrganization {
		return errors.Wrap(models.ConflictError, "an organization already exists on this instance")
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	if usesFirebase {
		if err := uc.firebase.CreateFirstUser(ctx, email, req.Password,
			fmt.Sprintf("%s %s", req.Firstname, req.Lastname)); err != nil {
			return errors.Wrap(err, "could not create Firebase user")
		}
	}

	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := repositories.GetAdvisoryLockTx(ctx, tx, initialOnboardingLockKey); err != nil {
			return errors.Wrap(err, "error getting advisory lock on initial onboarding")
		}

		hasAnOrganization, err := uc.orgRepository.HasOrganizations(ctx, tx)
		if err != nil {
			return err
		}
		if hasAnOrganization {
			return errors.Wrap(models.ConflictError, "an organization already exists on this instance")
		}

		orgId := pure_utils.NewId()

		if err := uc.orgRepository.CreateOrganization(ctx, tx, orgId,
			models.CreateOrganizationInput{Name: req.Organization}); err != nil {
			if repositories.IsUniqueViolationError(err) {
				return errors.Wrap(models.ConflictError, "organization with the same name already exists")
			}
			return err
		}

		userCreate := models.CreateUser{
			OrganizationId: orgId,
			Email:          email,
			FirstName:      req.Firstname,
			LastName:       req.Lastname,
			Role:           models.ADMIN,
		}

		userID, err := uc.userRepository.CreateUser(ctx, tx, userCreate)
		if err != nil {
			if repositories.IsUniqueViolationError(err) {
				return errors.Wrap(models.ConflictError, "user with the same email already exists")
			}
			return err
		}
		if err := uc.grantRepository.EnsureTenantAdminForOrganization(ctx, tx, userID, orgId); err != nil {
			return errors.Wrap(err, "could not create tenant admin grant")
		}

		return nil
	})
}
