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
	enforceSecurity  *mocks.EnforceSecurity
	inboxRepository  *mocks.InboxRepository
	userRepository   *mocks.UserRepository
	credentials      models.Credentials
	adminCredentials models.Credentials

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
		enforceSecurity: suite.enforceSecurity,
		inboxRepository: suite.inboxRepository,
		userRepository:  suite.userRepository,
		credentials:     suite.credentials,
	}
}

func (suite *InboxUsecaseTestSuite) makeUsecaseAdmin() *InboxUsecase {
	return &InboxUsecase{
		organizationIdOfContext: func() (string, error) {
			return suite.organizationId, nil
		},
		enforceSecurity: suite.enforceSecurity,
		inboxRepository: suite.inboxRepository,
		userRepository:  suite.userRepository,
		credentials:     suite.adminCredentials,
	}
}

func (suite *InboxUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.inboxRepository.AssertExpectations(t)
	suite.userRepository.AssertExpectations(t)
}

func (suite *InboxUsecaseTestSuite) Test_GetInboxById_nominal() {
	t := suite.T()

	suite.inboxRepository.On("GetInboxById", nil, suite.inboxId).Return(suite.inbox, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecase().GetInboxById(context.Background(), suite.inbox.Id)

	assert.NoError(t, err)
	assert.Equal(t, suite.inbox, result)

	suite.AssertExpectations()

}

func (suite *InboxUsecaseTestSuite) Test_GetInboxById_repository_error() {
	suite.inboxRepository.On("GetInboxById", nil, suite.inboxId).Return(models.Inbox{}, suite.repositoryError)

	_, err := suite.makeUsecase().GetInboxById(context.Background(), suite.inbox.Id)

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_GetInboxById_security_error() {
	suite.inboxRepository.On("GetInboxById", nil, suite.inboxId).Return(suite.inbox, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(suite.securityError)

	_, err := suite.makeUsecase().GetInboxById(context.Background(), suite.inbox.Id)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_ListInboxes_nominal_admin() {
	suite.inboxRepository.On("ListInboxes", nil, suite.organizationId, mock.MatchedBy(func(s []string) bool { return s == nil })).Return([]models.Inbox{suite.inbox}, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecaseAdmin().ListInboxes(context.Background())

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.Inbox{suite.inbox}, result)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_ListInboxes_nominal_non_admin() {
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{{InboxId: suite.inboxId}}, nil)
	suite.inboxRepository.On("ListInboxes", nil, suite.organizationId, []string{suite.inboxId}).Return([]models.Inbox{suite.inbox}, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(nil)

	result, err := suite.makeUsecase().ListInboxes(context.Background())

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.Inbox{suite.inbox}, result)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_ListInboxes_nominal_no_inboxes() {
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{}, nil)

	result, err := suite.makeUsecase().ListInboxes(context.Background())

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, []models.Inbox{}, result)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_ListInboxes_repository_error() {
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{{InboxId: suite.inboxId}}, nil)
	suite.inboxRepository.On("ListInboxes", nil, suite.organizationId, []string{suite.inboxId}).Return([]models.Inbox{}, suite.repositoryError)

	_, err := suite.makeUsecase().ListInboxes(context.Background())

	t := suite.T()
	assert.ErrorIs(t, err, suite.repositoryError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_ListInboxes_security_error() {
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{{InboxId: suite.inboxId}}, nil)
	suite.inboxRepository.On("ListInboxes", nil, suite.organizationId, []string{suite.inboxId}).Return([]models.Inbox{suite.inbox}, nil)
	suite.enforceSecurity.On("ReadInbox", suite.inbox).Return(suite.securityError)

	_, err := suite.makeUsecase().ListInboxes(context.Background())

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_nominal() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.enforceSecurity.On("CreateInbox", input).Return(nil)
	suite.inboxRepository.On("CreateInbox", nil, input, mock.AnythingOfType("string")).Return(nil)
	suite.inboxRepository.On("GetInboxById", nil, mock.AnythingOfType("string")).Return(suite.inbox, nil)

	inbox, err := suite.makeUsecaseAdmin().CreateInbox(context.Background(), input)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, suite.inbox, inbox)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_security_error() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.enforceSecurity.On("CreateInbox", input).Return(suite.securityError)

	_, err := suite.makeUsecase().CreateInbox(context.Background(), input)

	t := suite.T()
	assert.ErrorIs(t, err, suite.securityError)

	suite.AssertExpectations()
}

func (suite *InboxUsecaseTestSuite) Test_CreateInbox_repository_error() {
	input := models.CreateInboxInput{Name: "test inbox", OrganizationId: suite.organizationId}
	suite.enforceSecurity.On("CreateInbox", input).Return(nil)
	suite.inboxRepository.On("CreateInbox", nil, input, mock.AnythingOfType("string")).Return(suite.repositoryError)

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
	suite.inboxRepository.On("ListInboxUsers", nil, models.InboxUserFilterInput{UserId: suite.nonAdminUserId}).Return([]models.InboxUser{inboxUser}, nil)
	suite.inboxRepository.On("GetInboxById", nil, suite.inboxId).Return(targetInbox, nil).Return(targetInbox, nil)
	suite.inboxRepository.On("CreateInboxUser", nil, input, mock.AnythingOfType("string")).Return(nil)
	suite.enforceSecurity.On("CreateInboxUser", input, mock.AnythingOfType("[]models.InboxUser"), targetInbox, targetUser).Return(nil)
	suite.userRepository.On("UserByID", nil, suite.nonAdminUserId).Return(targetUser, nil)
	suite.inboxRepository.On("GetInboxUserById", nil, mock.AnythingOfType("string")).Return(inboxUser, nil)

	newInboxUser, err := suite.makeUsecase().CreateInboxUser(context.Background(), input)

	t := suite.T()
	assert.NoError(t, err)
	assert.Equal(t, newInboxUser, inboxUser)

	suite.AssertExpectations()
}

func TestInboxUsecase(t *testing.T) {
	suite.Run(t, new(InboxUsecaseTestSuite))
}
