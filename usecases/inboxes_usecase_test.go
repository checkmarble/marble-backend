package usecases

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/google/uuid"
)

type InboxUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity    *mocks.EnforceSecurity
	exec               *mocks.Executor
	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	executorFactory    *mocks.ExecutorFactory
	inboxRepository    *mocks.InboxRepository
	userRepository     *mocks.UserRepository
	credentials        models.Credentials
	adminCredentials   models.Credentials

	organizationId       uuid.UUID
	inboxId              string
	parsedInboxId        uuid.UUID
	inbox                models.Inbox
	adminUserId          string
	parsedAdminUserId    uuid.UUID
	nonAdminUserId       string
	parsedNonAdminUserId uuid.UUID
	repositoryError      error
	securityError        error
	ctx                  context.Context
}

func (suite *InboxUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.inboxRepository = new(mocks.InboxRepository)
	suite.userRepository = new(mocks.UserRepository)
	suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.executorFactory = new(mocks.ExecutorFactory)

	suite.organizationId = uuid.MustParse("25ab6323-1657-4a52-923a-ef6983fe4532")
	suite.inboxId = "0ae6fda7-f7b3-4218-9fc3-4efa329432a7"
	var err error
	suite.parsedInboxId, err = uuid.Parse(suite.inboxId)
	if err != nil {
		panic("failed to parse test inboxId: " + err.Error())
	}
	// Use valid UUID strings for user IDs that will be parsed
	suite.adminUserId = "a0000000-0000-0000-0000-000000000001"
	suite.parsedAdminUserId, err = uuid.Parse(suite.adminUserId)
	if err != nil {
		panic("failed to parse test adminUserId: " + err.Error())
	}
	suite.nonAdminUserId = "a0000000-0000-0000-0000-000000000002"
	suite.parsedNonAdminUserId, err = uuid.Parse(suite.nonAdminUserId)
	if err != nil {
		panic("failed to parse test nonAdminUserId: " + err.Error())
	}
	suite.inbox = models.Inbox{
		Id:             suite.parsedInboxId, // Use parsed UUID
		OrganizationId: suite.organizationId,
		Name:           "test inbox",
	}
	suite.credentials = models.Credentials{
		ActorIdentity: models.Identity{
			UserId: models.UserId(suite.nonAdminUserId),
		},
		OrganizationId: suite.organizationId,
		Role:           models.BUILDER,
	}
	suite.adminCredentials = models.Credentials{
		ActorIdentity: models.Identity{
			UserId: models.UserId(suite.adminUserId),
		},
		OrganizationId: suite.organizationId,
		Role:           models.ADMIN,
	}
	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
	suite.ctx = context.Background()
}

func (suite *InboxUsecaseTestSuite) makeUsecase() *InboxUsecase {
	return &InboxUsecase{
		enforceSecurity:    suite.enforceSecurity,
		inboxRepository:    suite.inboxRepository,
		userRepository:     suite.userRepository,
		credentials:        suite.credentials,
		transactionFactory: suite.transactionFactory,
		executorFactory:    suite.executorFactory,
		inboxUsers: inboxes.InboxUsers{
			EnforceSecurity:     suite.enforceSecurity,
			InboxUserRepository: suite.inboxRepository,
			Credentials:         suite.credentials,
			TransactionFactory:  suite.transactionFactory,
			ExecutorFactory:     suite.executorFactory,
			UserRepository:      suite.userRepository,
		},
	}
}

func (suite *InboxUsecaseTestSuite) makeUsecaseAdmin() *InboxUsecase {
	return &InboxUsecase{
		enforceSecurity:    suite.enforceSecurity,
		inboxRepository:    suite.inboxRepository,
		userRepository:     suite.userRepository,
		credentials:        suite.adminCredentials,
		transactionFactory: suite.transactionFactory,
		executorFactory:    suite.executorFactory,
		inboxUsers: inboxes.InboxUsers{
			EnforceSecurity:     suite.enforceSecurity,
			InboxUserRepository: suite.inboxRepository,
			Credentials:         suite.credentials,
			TransactionFactory:  suite.transactionFactory,
			ExecutorFactory:     suite.executorFactory,
			UserRepository:      suite.userRepository,
		},
	}
}

func (suite *InboxUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.inboxRepository.AssertExpectations(t)
	suite.userRepository.AssertExpectations(t)
	suite.exec.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_nominal() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateInbox", suite.organizationId).Return(nil)
	suite.inboxRepository.On("CreateInbox", suite.ctx, suite.transaction, input,
		mock.AnythingOfType("uuid.UUID")).Return(nil)
	suite.inboxRepository.On("GetInboxById", suite.ctx, suite.transaction,
		mock.AnythingOfType("uuid.UUID")).Return(suite.inbox, nil)

	inbox, err := suite.makeUsecaseAdmin().CreateInbox(suite.ctx, input)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.inbox, inbox)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_security_error() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateInbox", suite.organizationId).Return(suite.securityError)

	_, err := suite.makeUsecase().CreateInbox(suite.ctx, input)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_repository_error() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateInbox", suite.organizationId).Return(nil)
	suite.inboxRepository.On("CreateInbox", suite.ctx, suite.transaction, input,
		mock.AnythingOfType("uuid.UUID")).Return(suite.repositoryError)

	_, err := suite.makeUsecaseAdmin().CreateInbox(suite.ctx, input)

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_GetInboxUserById_nominal() {
	inboxUser := models.InboxUser{
		InboxId: suite.parsedInboxId,
		UserId:  suite.parsedNonAdminUserId,
	}
	parsedTestInboxUserId := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("GetInboxUserById", suite.ctx, suite.transaction,
		parsedTestInboxUserId).Return(inboxUser, nil)
	suite.inboxRepository.On("ListInboxUsers", suite.ctx, suite.transaction, models.InboxUserFilterInput{
		UserId: models.UserId(suite.nonAdminUserId),
	}).Return([]models.InboxUser{inboxUser}, nil)
	suite.enforceSecurity.On("ReadInboxUser", inboxUser,
		mock.AnythingOfType("[]models.InboxUser")).Return(nil)

	result, err := suite.makeUsecase().GetInboxUserById(context.Background(), parsedTestInboxUserId)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, inboxUser, result)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_GetInboxUserById_security_error() {
	inboxUser := models.InboxUser{
		InboxId: suite.parsedInboxId,
		UserId:  suite.parsedNonAdminUserId,
	}
	parsedTestInboxUserId := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("GetInboxUserById", suite.ctx, suite.transaction,
		parsedTestInboxUserId).Return(inboxUser, nil)
	suite.inboxRepository.On("ListInboxUsers", suite.ctx, suite.transaction, models.InboxUserFilterInput{
		UserId: models.UserId(suite.nonAdminUserId),
	}).Return([]models.InboxUser{inboxUser}, nil)
	suite.enforceSecurity.On("ReadInboxUser", inboxUser, []models.InboxUser{inboxUser}).Return(suite.securityError)

	_, err := suite.makeUsecase().GetInboxUserById(context.Background(), parsedTestInboxUserId)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_ListInboxUsers_nominal() {
	inboxUser := models.InboxUser{
		InboxId: suite.parsedInboxId,
		UserId:  suite.parsedNonAdminUserId,
	}
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("ListInboxUsers", suite.ctx, suite.transaction, models.InboxUserFilterInput{
		InboxId: suite.parsedInboxId,
	}).Return([]models.InboxUser{inboxUser}, nil)
	suite.inboxRepository.On("ListInboxUsers", suite.ctx, suite.transaction, models.InboxUserFilterInput{
		UserId: models.UserId(suite.nonAdminUserId),
	}).Return([]models.InboxUser{inboxUser}, nil)
	suite.enforceSecurity.On("ReadInboxUser", inboxUser, []models.InboxUser{inboxUser}).Return(nil)

	result, err := suite.makeUsecase().ListInboxUsers(context.Background(), suite.parsedInboxId)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.InboxUser{inboxUser}, result)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInboxUser_nominal_non_admin() {
	inboxUser := models.InboxUser{
		InboxId: suite.parsedInboxId,
		UserId:  suite.parsedNonAdminUserId, Role: models.InboxUserRoleAdmin,
	}
	input := models.CreateInboxUserInput{
		InboxId: suite.parsedInboxId,
		UserId:  suite.parsedNonAdminUserId, Role: models.InboxUserRoleAdmin,
	}

	targetUser := models.User{
		OrganizationId: suite.organizationId,
		UserId:         models.UserId(suite.nonAdminUserId),
	}
	targetInbox := models.Inbox{OrganizationId: suite.organizationId, Id: suite.parsedInboxId}
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateInboxUser", input,
		mock.AnythingOfType("[]models.InboxUser"), targetInbox, targetUser).Return(nil)
	suite.inboxRepository.On("ListInboxUsers", suite.ctx, suite.transaction, models.InboxUserFilterInput{
		UserId: models.UserId(suite.nonAdminUserId),
	}).Return([]models.InboxUser{inboxUser}, nil)
	suite.inboxRepository.On("GetInboxById", suite.ctx, suite.transaction, suite.parsedInboxId).Return(
		targetInbox, nil).Return(targetInbox, nil)
	suite.inboxRepository.On("CreateInboxUser", suite.ctx, suite.transaction, input,
		mock.AnythingOfType("uuid.UUID")).Return(nil)
	suite.inboxRepository.On("GetInboxUserById", suite.ctx, suite.transaction,
		mock.AnythingOfType("uuid.UUID")).Return(inboxUser, nil)
	suite.userRepository.On("UserById", suite.ctx, suite.transaction, suite.nonAdminUserId).Return(
		targetUser, nil)

	newInboxUser, err := suite.makeUsecase().CreateInboxUser(suite.ctx, input)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, newInboxUser, inboxUser)

	suite.AssertExpectations()
}

func TestInboxUsecase(t *testing.T) {
	suite.Run(t, new(InboxUsecaseTestSuite))
}
