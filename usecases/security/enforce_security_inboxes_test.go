package security_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/security"
)

func Test_ReadInbox(t *testing.T) {
	organizationId := "orgId"
	anotherOrganizationId := "anotherOrgId"
	userId := models.UserId("userId")
	actorIdentity := models.Identity{
		UserId: userId,
	}
	t.Run("admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.ADMIN, OrganizationId: organizationId}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("right org", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: organizationId})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("wrong org", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: anotherOrganizationId})
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})

	t.Run("Marble admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.MARBLE_ADMIN, OrganizationId: organizationId}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("right org", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: organizationId})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("wrong org", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: anotherOrganizationId})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	})

	t.Run("non admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.BUILDER, OrganizationId: organizationId, ActorIdentity: actorIdentity}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("User is member of the inbox", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: organizationId, InboxUsers: []models.InboxUser{{UserId: "userId"}}})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("User is not member of the inbox", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: organizationId})
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})
}

func Test_CreateInbox(t *testing.T) {
	organizationId := "orgId"
	anotherOrganizationId := "anotherOrgId"
	userId := models.UserId("userId")
	actorIdentity := models.Identity{
		UserId: userId,
	}

	t.Run("admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.ADMIN, OrganizationId: organizationId}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("creating an inbox in the same org should succeed", func(t *testing.T) {
			err := sec.CreateInbox(organizationId)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("creating an inbox in a different org should fail", func(t *testing.T) {
			err := sec.CreateInbox(anotherOrganizationId)
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})

	t.Run("non admin: creating an inbox in the same org should fail", func(t *testing.T) {
		creds := models.Credentials{Role: models.BUILDER, OrganizationId: organizationId, ActorIdentity: actorIdentity}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		err := sec.CreateInbox(organizationId)
		if err == nil {
			t.Errorf("Expected error, got nil")
		}
	})
}

func Test_ReadInboxUser(t *testing.T) {
	organizationId := "orgId"
	anotherOrganizationId := "anotherOrgId"
	userId := models.UserId("userId")
	actorIdentity := models.Identity{
		UserId: userId,
	}
	inboxId := "inboxId"

	t.Run("admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.ADMIN, OrganizationId: organizationId}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to read any inbox user from the org", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{InboxId: inboxId, OrganizationId: organizationId}, nil)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("Should not be able to read any inbox user from another org", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{InboxId: inboxId, OrganizationId: anotherOrganizationId}, nil)
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})

	t.Run("non admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.BUILDER, OrganizationId: organizationId, ActorIdentity: actorIdentity}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to read an inbox user if the calling user is member of the inbox", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{InboxId: inboxId}, []models.InboxUser{{InboxId: inboxId}})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("Should not be able to read an inbox user if the calling user is not a member of the inbox", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{InboxId: inboxId}, []models.InboxUser{{InboxId: "anotherInboxId"}})
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})
}

func Test_CreateInboxUser(t *testing.T) {
	t.Run("admin", func(t *testing.T) {
		organizationId := "orgId"
		anotherOrganizationId := "anotherOrgId"
		creds := models.Credentials{Role: models.ADMIN, OrganizationId: organizationId}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to create an inbox user in any inbox", func(t *testing.T) {
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: "inboxId"}, nil, models.Inbox{OrganizationId: organizationId}, models.User{OrganizationId: organizationId},
			)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in another org", func(t *testing.T) {
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: "inboxId"}, nil, models.Inbox{OrganizationId: anotherOrganizationId}, models.User{OrganizationId: organizationId},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})
	})

	t.Run("non admin", func(t *testing.T) {
		inboxId := "inboxId"
		organizationId := "orgId"
		anotherOrganizationId := "anotherOrgId"
		userId := models.UserId("userId")
		actorIdentity := models.Identity{
			UserId: userId,
		}
		creds := models.Credentials{Role: models.BUILDER, OrganizationId: organizationId, ActorIdentity: actorIdentity}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to create an inbox user in an inbox if the calling user is admin of the inbox", func(t *testing.T) {
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: inboxId},
				[]models.InboxUser{{InboxId: inboxId, Role: models.InboxUserRoleAdmin}},
				models.Inbox{OrganizationId: organizationId},
				models.User{OrganizationId: organizationId},
			)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in an inbox if the calling user is not member of the inbox", func(t *testing.T) {
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: inboxId},
				[]models.InboxUser{{InboxId: "anotherInboxId", Role: models.InboxUserRoleAdmin}},
				models.Inbox{OrganizationId: organizationId},
				models.User{OrganizationId: organizationId},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in an inbox if the calling user is not admin of the inbox", func(t *testing.T) {
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: inboxId},
				[]models.InboxUser{{InboxId: inboxId, Role: models.InboxUserRoleMember}},
				models.Inbox{OrganizationId: organizationId},
				models.User{OrganizationId: organizationId},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user if the target user does not belong to the right org", func(t *testing.T) {
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: inboxId},
				[]models.InboxUser{{InboxId: inboxId}},
				models.Inbox{OrganizationId: organizationId},
				models.User{OrganizationId: anotherOrganizationId},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})
	})
}
