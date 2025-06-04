package inboxes

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type InboxReaderTestSuite struct {
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

	organizationId  string
	inboxId         string // string version
	parsedInboxId   uuid.UUID // uuid version
	inbox           models.Inbox
	adminUserId     models.UserId
	nonAdminUserId  models.UserId
	repositoryError error
	securityError   error
	ctx             context.Context
}

func (suite *InboxReaderTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.inboxRepository = new(mocks.InboxRepository)
	suite.userRepository = new(mocks.UserRepository)
	suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.executorFactory = new(mocks.ExecutorFactory)

	suite.ctx = context.Background()

	suite.organizationId = "25ab6323-1657-4a52-923a-ef6983fe4532"
	suite.inboxId = "0ae6fda7-f7b3-4218-9fc3-4efa329432a7"
	var err error
	suite.parsedInboxId, err = uuid.Parse(suite.inboxId)
	if err != nil {
		panic("failed to parse test inboxId: " + err.Error())
	}
	suite.adminUserId = models.UserId("some user id")
	suite.nonAdminUserId = models.UserId("some other user id")
	suite.inbox = models.Inbox{
		Id:             suite.parsedInboxId, // Use parsed UUID
		OrganizationId: suite.organizationId,
		Name:           "test inbox",
	}
	suite.credentials = models.Credentials{
		ActorIdentity: models.Identity{
			UserId: suite.nonAdminUserId,
		},
		OrganizationId: suite.organizationId,
		Role:           models.BUILDER,
	}
	suite.adminCredentials = models.Credentials{
		ActorIdentity: models.Identity{
			UserId: suite.adminUserId,
		},
		OrganizationId: suite.organizationId,
		Role:           models.ADMIN,
	}
	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
}

func (suite *InboxReaderTestSuite) makeUsecase() *InboxReader {
	return &InboxReader{
		EnforceSecurity: suite.enforceSecurity,
		InboxRepository: suite.inboxRepository,
		Credentials:     suite.credentials,
		ExecutorFactory: suite.executorFactory,
	}
}

func (suite *InboxReaderTestSuite) makeUsecaseAdmin() *InboxReader {
	return &InboxReader{
		EnforceSecurity: suite.enforceSecurity,
		InboxRepository: suite.inboxRepository,
		Credentials:     suite.adminCredentials,
		ExecutorFactory: suite.executorFactory,
	}
}

func (suite *InboxReaderTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.inboxRepository.AssertExpectations(t)
	suite.userRepository.AssertExpectations(t)
	suite.exec.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
}

func (suite *InboxReaderTestSuite) Test_GetInboxById_nominal() {
	t := suite.T()

	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("GetInboxById", suite.transaction, suite.parsedInboxId).Return(suite.inbox, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecase().GetInboxById(suite.ctx, suite.parsedInboxId) // Pass parsed UUID

	assert.NoError(t, err)
	assert.Equal(t, suite.inbox, result)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_GetInboxById_repository_error() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("GetInboxById", suite.transaction, suite.parsedInboxId).Return(models.Inbox{}, suite.repositoryError)

	_, err := suite.makeUsecase().GetInboxById(suite.ctx, suite.parsedInboxId) // Pass parsed UUID

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_GetInboxById_security_error() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("GetInboxById", suite.transaction, suite.parsedInboxId).Return(suite.inbox, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(suite.securityError)

	_, err := suite.makeUsecase().GetInboxById(suite.ctx, suite.parsedInboxId) // Pass parsed UUID

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_nominal_admin() {
	suite.inboxRepository.On("ListInboxes", suite.transaction, suite.organizationId,
		mock.MatchedBy(func(s []uuid.UUID) bool { return s == nil })).Return([]models.Inbox{suite.inbox}, nil) // Expect []uuid.UUID
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecaseAdmin().ListInboxes(suite.ctx, suite.transaction, suite.organizationId, false)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.Inbox{suite.inbox}, result)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_nominal_non_admin() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("ListInboxUsers", suite.transaction, models.InboxUserFilterInput{
		UserId: suite.nonAdminUserId, // UserId in filter is string
	}).Return([]models.InboxUser{{InboxId: suite.parsedInboxId}}, nil) // InboxUser.InboxId is uuid.UUID
	suite.inboxRepository.On("ListInboxes", suite.transaction, suite.organizationId, []uuid.UUID{ // Expect []uuid.UUID
		suite.parsedInboxId,
	}).Return([]models.Inbox{suite.inbox}, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecase().ListInboxes(suite.ctx, suite.transaction, suite.organizationId, false)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.Inbox{suite.inbox}, result)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_nominal_no_inboxes() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("ListInboxUsers", suite.transaction, models.InboxUserFilterInput{
		UserId: suite.nonAdminUserId,
	}).Return([]models.InboxUser{}, nil)

	result, err := suite.makeUsecase().ListInboxes(suite.ctx, suite.transaction, suite.organizationId, false)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.Inbox{}, result)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_repository_error() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("ListInboxUsers", suite.transaction, models.InboxUserFilterInput{
		UserId: suite.nonAdminUserId, // UserId in filter is string
	}).Return([]models.InboxUser{{InboxId: suite.parsedInboxId}}, nil) // InboxUser.InboxId is uuid.UUID
	suite.inboxRepository.On("ListInboxes", suite.transaction, suite.organizationId, []uuid.UUID{ // Expect []uuid.UUID
		suite.parsedInboxId,
	}).Return([]models.Inbox{}, suite.repositoryError)

	_, err := suite.makeUsecase().ListInboxes(suite.ctx, suite.transaction, suite.organizationId, false)

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_security_error() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("ListInboxUsers", suite.transaction, models.InboxUserFilterInput{
		UserId: suite.nonAdminUserId, // UserId in filter is string
	}).Return([]models.InboxUser{{InboxId: suite.parsedInboxId}}, nil) // InboxUser.InboxId is uuid.UUID
	suite.inboxRepository.On("ListInboxes", suite.transaction, suite.organizationId, []uuid.UUID{ // Expect []uuid.UUID
		suite.parsedInboxId,
	}).Return([]models.Inbox{suite.inbox}, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(suite.securityError)

	_, err := suite.makeUsecase().ListInboxes(suite.ctx, suite.transaction, suite.organizationId, false)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func TestInboxReader(t *testing.T) {
	suite.Run(t, new(InboxReaderTestSuite))
}
