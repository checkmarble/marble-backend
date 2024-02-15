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
)

type InboxReaderTestSuite struct {
	suite.Suite
	enforceSecurity    *mocks.EnforceSecurity
	transaction        *mocks.Executor
	transactionFactory *mocks.TransactionFactory
	executorFactory    *mocks.ExecutorFactory
	inboxRepository    *mocks.InboxRepository
	userRepository     *mocks.UserRepository
	credentials        models.Credentials
	adminCredentials   models.Credentials

	organizationId  string
	inboxId         string
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
	suite.transaction = new(mocks.Executor)
	suite.transactionFactory = &mocks.TransactionFactory{ExecMock: suite.transaction}
	suite.executorFactory = new(mocks.ExecutorFactory)

	suite.ctx = context.Background()

	suite.organizationId = "25ab6323-1657-4a52-923a-ef6983fe4532"
	suite.inboxId = "0ae6fda7-f7b3-4218-9fc3-4efa329432a7"
	suite.adminUserId = models.UserId("some user id")
	suite.nonAdminUserId = models.UserId("some other user id")
	suite.inbox = models.Inbox{
		Id:             suite.inboxId,
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
		OrganizationIdOfContext: func() (string, error) {
			return suite.organizationId, nil
		},
		EnforceSecurity: suite.enforceSecurity,
		InboxRepository: suite.inboxRepository,
		Credentials:     suite.credentials,
		ExecutorFactory: suite.executorFactory,
	}
}

func (suite *InboxReaderTestSuite) makeUsecaseAdmin() *InboxReader {
	return &InboxReader{
		OrganizationIdOfContext: func() (string, error) {
			return suite.organizationId, nil
		},
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
}

func (suite *InboxReaderTestSuite) Test_GetInboxById_nominal() {
	t := suite.T()

	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("GetInboxById", suite.transaction, suite.inboxId).Return(suite.inbox, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecase().GetInboxById(suite.ctx, suite.inbox.Id)

	assert.NoError(t, err)
	assert.Equal(t, suite.inbox, result)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_GetInboxById_repository_error() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("GetInboxById", suite.transaction, suite.inboxId).Return(models.Inbox{}, suite.repositoryError)

	_, err := suite.makeUsecase().GetInboxById(suite.ctx, suite.inbox.Id)

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_GetInboxById_security_error() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("GetInboxById", suite.transaction, suite.inboxId).Return(suite.inbox, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(suite.securityError)

	_, err := suite.makeUsecase().GetInboxById(suite.ctx, suite.inbox.Id)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_nominal_admin() {
	suite.inboxRepository.On("ListInboxes", suite.transaction, suite.organizationId,
		mock.MatchedBy(func(s []string) bool { return s == nil })).Return([]models.Inbox{suite.inbox}, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecaseAdmin().ListInboxes(suite.ctx, suite.transaction, false)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.Inbox{suite.inbox}, result)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_nominal_non_admin() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("ListInboxUsers", suite.transaction, models.InboxUserFilterInput{
		UserId: suite.nonAdminUserId,
	}).Return([]models.InboxUser{{InboxId: suite.inboxId}}, nil)
	suite.inboxRepository.On("ListInboxes", suite.transaction, suite.organizationId, []string{
		suite.inboxId,
	}).Return([]models.Inbox{suite.inbox}, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecase().ListInboxes(suite.ctx, suite.transaction, false)

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

	result, err := suite.makeUsecase().ListInboxes(suite.ctx, suite.transaction, false)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.Inbox{}, result)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_repository_error() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("ListInboxUsers", suite.transaction, models.InboxUserFilterInput{
		UserId: suite.nonAdminUserId,
	}).Return([]models.InboxUser{{InboxId: suite.inboxId}}, nil)
	suite.inboxRepository.On("ListInboxes", suite.transaction, suite.organizationId, []string{
		suite.inboxId,
	}).Return([]models.Inbox{}, suite.repositoryError)

	_, err := suite.makeUsecase().ListInboxes(suite.ctx, suite.transaction, false)

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func (suite *InboxReaderTestSuite) Test_ListInboxes_security_error() {
	suite.executorFactory.On("NewExecutor").Once().Return(suite.transaction)
	suite.inboxRepository.On("ListInboxUsers", suite.transaction, models.InboxUserFilterInput{
		UserId: suite.nonAdminUserId,
	}).Return([]models.InboxUser{{InboxId: suite.inboxId}}, nil)
	suite.inboxRepository.On("ListInboxes", suite.transaction, suite.organizationId, []string{
		suite.inboxId,
	}).Return([]models.Inbox{suite.inbox}, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(suite.securityError)

	_, err := suite.makeUsecase().ListInboxes(suite.ctx, suite.transaction, false)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func TestInboxReader(t *testing.T) {
	suite.Run(t, new(InboxReaderTestSuite))
}
