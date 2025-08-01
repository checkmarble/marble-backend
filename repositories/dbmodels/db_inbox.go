package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

// Inboxes

type DBInbox struct {
	Id                uuid.UUID  `db:"id"`
	OrganizationId    string     `db:"organization_id"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
	Name              string     `db:"name"`
	Status            string     `db:"status"`
	EscalationInboxId *uuid.UUID `db:"escalation_inbox_id"`
	AutoAssignEnabled bool       `db:"auto_assign_enabled"`
}

const TABLE_INBOXES = "inboxes"

var SelectInboxColumn = utils.ColumnList[DBInbox]()

func AdaptInbox(db DBInbox) (models.Inbox, error) {
	return models.Inbox{
		Id:                db.Id,
		OrganizationId:    db.OrganizationId,
		CreatedAt:         db.CreatedAt,
		UpdatedAt:         db.UpdatedAt,
		Name:              db.Name,
		Status:            models.InboxStatus(db.Status),
		EscalationInboxId: db.EscalationInboxId,
		AutoAssignEnabled: db.AutoAssignEnabled,
	}, nil
}

// Inbox users

type DBInboxUser struct {
	Id             uuid.UUID `db:"id"`
	InboxId        uuid.UUID `db:"inbox_id"`
	UserId         uuid.UUID `db:"user_id"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Role           string    `db:"role"`
	AutoAssignable bool      `db:"auto_assignable"`
}

type DBInboxUserWithOrgId struct {
	DBInboxUser
	OrganizationId string `db:"organization_id"`
}

const TABLE_INBOX_USERS = "inbox_users"

var (
	SelectInboxUserColumn          = utils.ColumnList[DBInboxUser]()
	SelectInboxUserWithOrgIdColumn = utils.ColumnList[DBInboxUser]()
)

func AdaptInboxUser(db DBInboxUser) (models.InboxUser, error) {
	return models.InboxUser{
		Id:             db.Id,
		InboxId:        db.InboxId,
		UserId:         db.UserId,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
		Role:           models.InboxUserRole(db.Role),
		AutoAssignable: db.AutoAssignable,
	}, nil
}

func AdaptInboxUserWithOrgId(db DBInboxUserWithOrgId) (models.InboxUser, error) {
	inboxUser, _ := AdaptInboxUser(db.DBInboxUser)
	inboxUser.OrganizationId = db.OrganizationId
	return inboxUser, nil
}

type DBInboxWithUsers struct {
	DBInbox
	InboxUsers []DBInboxUser `db:"inbox_users"`
}

func AdaptInboxWithUsers(db DBInboxWithUsers) (models.Inbox, error) {
	inbox, err := AdaptInbox(db.DBInbox)
	if err != nil {
		return models.Inbox{}, err
	}

	inboxUsers := make([]models.InboxUser, len(db.InboxUsers))
	for i, inboxUser := range db.InboxUsers {
		inboxUsers[i], err = AdaptInboxUser(inboxUser)
		inboxUsers[i].OrganizationId = inbox.OrganizationId
		if err != nil {
			return models.Inbox{}, err
		}
	}

	inbox.InboxUsers = inboxUsers
	return inbox, nil
}

type DBInboxWithDetails struct {
	DBInboxWithUsers
	CasesCount int `db:"cases_count"`
}

func AdaptInboxWithCasesCount(db DBInboxWithDetails) (models.Inbox, error) {
	inbox, err := AdaptInboxWithUsers(db.DBInboxWithUsers)
	if err != nil {
		return models.Inbox{}, err
	}

	inbox.CasesCount = &db.CasesCount
	return inbox, nil
}
