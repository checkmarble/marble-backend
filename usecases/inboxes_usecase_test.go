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
)

type InboxUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity    *mocks.EnforceSecurity
	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
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
}

func (suite *InboxUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.inboxRepository = new(mocks.InboxRepository)
	suite.userRepository = new(mocks.UserRepository)
	suite.transaction = new(mocks.Transaction)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}

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

func (suite *InboxUsecaseTestSuite) makeUsecase() *InboxUsecase {
	return &InboxUsecase{
		organizationIdOfContext: func() (string, error) {
			return suite.organizationId, nil
		},
		enforceSecurity:    suite.enforceSecurity,
		inboxRepository:    suite.inboxRepository,
		userRepository:     suite.userRepository,
		credentials:        suite.credentials,
		transactionFactory: suite.transactionFactory,
	}
}

func (suite *InboxUsecaseTestSuite) makeUsecaseAdmin() *InboxUsecase {
	return &InboxUsecase{
		organizationIdOfContext: func() (string, error) {
			return suite.organizationId, nil
		},
		enforceSecurity:    suite.enforceSecurity,
		inboxRepository:    suite.inboxRepository,
		userRepository:     suite.userRepository,
		credentials:        suite.adminCredentials,
		transactionFactory: suite.transactionFactory,
	}
}

func (suite *InboxUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.inboxRepository.AssertExpectations(t)
	suite.userRepository.AssertExpectations(t)
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_nominal() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateInbox", input).Return(nil)
	suite.inboxRepository.On("CreateInbox", suite.transaction, input, mock.AnythingOfType("string")).Return(nil)
	suite.inboxRepository.On("GetInboxById", suite.transaction, mock.AnythingOfType("string")).Return(suite.inbox, nil)

	inbox, err := suite.makeUsecaseAdmin().CreateInbox(context.Background(), input)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.inbox, inbox)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_security_error() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateInbox", input).Return(suite.securityError)

	_, err := suite.makeUsecase().CreateInbox(context.Background(), input)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_repository_error() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateInbox", input).Return(nil)
	suite.inboxRepository.On("CreateInbox", suite.transaction, input, mock.AnythingOfType("string")).Return(suite.repositoryError)

	_, err := suite.makeUsecaseAdmin().CreateInbox(context.Background(), input)

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_GetInboxUserById_nominal() {
	inboxUser := models.InboxUser{InboxId: suite.inboxId}
	suite.inboxRepository.On("GetInboxUserById", nil, mock.AnythingOfType("string")).Return(inboxUser, nil)
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{inboxUser}, nil)
	suite.enforceSecurity.On("ReadInboxUser", inboxUser, mock.AnythingOfType("[]models.InboxUser")).Return(nil)

	result, err := suite.makeUsecase().GetInboxUserById(context.Background(), "some inbox user id")

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, inboxUser, result)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_GetInboxUserById_security_error() {
	inboxUser := models.InboxUser{InboxId: suite.inboxId}
	suite.inboxRepository.On("GetInboxUserById", nil, mock.AnythingOfType("string")).Return(inboxUser, nil)
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{inboxUser}, nil)
	suite.enforceSecurity.On("ReadInboxUser", inboxUser, []models.InboxUser{inboxUser}).Return(suite.securityError)

	_, err := suite.makeUsecase().GetInboxUserById(context.Background(), "some inbox user id")

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_ListInboxUsers_nominal() {
	inboxUser := models.InboxUser{InboxId: suite.inboxId}
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{InboxId: suite.inboxId}).Return([]models.InboxUser{inboxUser}, nil)
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{inboxUser}, nil)
	suite.enforceSecurity.On("ReadInboxUser", inboxUser, []models.InboxUser{inboxUser}).Return(nil)

	result, err := suite.makeUsecase().ListInboxUsers(context.Background(), suite.inboxId)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.InboxUser{inboxUser}, result)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInboxUser_nominal_non_admin() {
	inboxUser := models.InboxUser{InboxId: suite.inboxId, UserId: string(suite.nonAdminUserId), Role: models.InboxUserRoleAdmin}
	input := models.CreateInboxUserInput{InboxId: suite.inboxId, UserId: string(suite.nonAdminUserId), Role: models.InboxUserRoleAdmin}
	targetUser := models.User{OrganizationId: suite.organizationId, UserId: suite.nonAdminUserId}
	targetInbox := models.Inbox{OrganizationId: suite.organizationId, Id: suite.inboxId}
	suite.transactionFactory.On("Transaction", models.DATABASE_MARBLE_SCHEMA, mock.Anything).Return(nil)
	suite.enforceSecurity.On("CreateInboxUser", input, mock.AnythingOfType("[]models.InboxUser"), targetInbox, targetUser).Return(nil)
	suite.inboxRepository.On("ListInboxUsers", suite.transaction, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{inboxUser}, nil)
	suite.inboxRepository.On("GetInboxById", suite.transaction, suite.inboxId).Return(targetInbox, nil).Return(targetInbox, nil)
	suite.inboxRepository.On("CreateInboxUser", suite.transaction, input, mock.AnythingOfType("string")).Return(nil)
	suite.inboxRepository.On("GetInboxUserById", suite.transaction, mock.AnythingOfType("string")).Return(inboxUser, nil)
	suite.userRepository.On("UserByID", suite.transaction, suite.nonAdminUserId).Return(targetUser, nil)

	newInboxUser, err := suite.makeUsecase().CreateInboxUser(context.Background(), input)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, newInboxUser, inboxUser)

	suite.AssertExpectations()
}

func TestInboxUsecase(t *testing.T) {
	suite.Run(t, new(InboxUsecaseTestSuite))
}
