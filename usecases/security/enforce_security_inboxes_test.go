package security_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

func Test_ReadInbox(t *testing.T) {
	organizationId := "orgId"
	anotherOrganizationId := "anotherOrgId"
	orgIdUUID := utils.TextToUUID(organizationId)
	anotherOrgIdUUID := utils.TextToUUID(anotherOrganizationId)

	t.Run("admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.ADMIN, OrganizationId: orgIdUUID}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("right org", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: orgIdUUID})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("wrong org", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: anotherOrgIdUUID})
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})

	t.Run("Marble admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.MARBLE_ADMIN, OrganizationId: orgIdUUID}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("right org", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: orgIdUUID})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("wrong org", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: anotherOrgIdUUID})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	})

	t.Run("non admin", func(t *testing.T) {
		actorUserIdString := "00000000-0000-0000-0000-000000000000"
		actorParsedUUID := uuid.MustParse(actorUserIdString)
		specificActorIdentity := models.Identity{UserId: models.UserId(actorUserIdString)}

		creds := models.Credentials{Role: models.BUILDER, OrganizationId: orgIdUUID, ActorIdentity: specificActorIdentity}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("User is member of the inbox", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{
				OrganizationId: orgIdUUID,
				InboxUsers:     []models.InboxUser{{UserId: actorParsedUUID}},
			})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("User is not member of the inbox", func(t *testing.T) {
			err := sec.ReadInbox(models.Inbox{OrganizationId: orgIdUUID})
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})
}

func Test_CreateInbox(t *testing.T) {
	organizationId := "orgId"
	anotherOrganizationId := "anotherOrgId"
	orgIdUUID := utils.TextToUUID(organizationId)
	anotherOrgIdUUID := utils.TextToUUID(anotherOrganizationId)
	userId := models.UserId("userId")
	actorIdentity := models.Identity{
		UserId: userId,
	}

	t.Run("admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.ADMIN, OrganizationId: orgIdUUID}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("creating an inbox in the same org should succeed", func(t *testing.T) {
			err := sec.CreateInbox(orgIdUUID)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("creating an inbox in a different org should fail", func(t *testing.T) {
			err := sec.CreateInbox(anotherOrgIdUUID)
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})

	t.Run("non admin: creating an inbox in the same org should fail", func(t *testing.T) {
		creds := models.Credentials{Role: models.BUILDER, OrganizationId: orgIdUUID, ActorIdentity: actorIdentity}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		err := sec.CreateInbox(orgIdUUID)
		if err == nil {
			t.Errorf("Expected error, got nil")
		}
	})
}

func Test_ReadInboxUser(t *testing.T) {
	organizationId := "orgId"
	anotherOrganizationId := "anotherOrgId"
	orgIdUUID := utils.TextToUUID(organizationId)
	anotherOrgIdUUID := utils.TextToUUID(anotherOrganizationId)
	userId := models.UserId("userId")
	actorIdentity := models.Identity{
		UserId: userId,
	}

	inboxId1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	inboxId2 := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	t.Run("admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.ADMIN, OrganizationId: orgIdUUID}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to read any inbox user from the org", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{InboxId: inboxId1, OrganizationId: orgIdUUID}, nil)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("Should not be able to read any inbox user from another org", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{
				InboxId:        inboxId1,
				OrganizationId: anotherOrgIdUUID,
			}, nil)
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		})
	})

	t.Run("non admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.BUILDER, OrganizationId: orgIdUUID, ActorIdentity: actorIdentity}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to read an inbox user if the calling user is member of the inbox", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{InboxId: inboxId1}, []models.InboxUser{
				{InboxId: inboxId1},
			})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("Should not be able to read an inbox user if the calling user is not a member of the inbox", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{InboxId: inboxId1}, []models.InboxUser{
				{InboxId: inboxId2},
			})
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
		adminTestInboxId := uuid.MustParse("00000000-0000-0000-0000-000000000000")

		creds := models.Credentials{
			Role:           models.ADMIN,
			OrganizationId: utils.TextToUUID(organizationId),
		}
		orgIdUUIDString := creds.OrganizationId
		anotherOrgIdUUID := utils.TextToUUID(anotherOrganizationId)
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to create an inbox user in any inbox", func(t *testing.T) {
			userIdString := "11111111-1111-1111-1111-111111111111"
			createInputTargetUserId := uuid.MustParse(userIdString)
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{
					InboxId: adminTestInboxId,
					UserId:  createInputTargetUserId,
				}, nil, models.Inbox{
					Id: adminTestInboxId, OrganizationId: orgIdUUIDString,
				}, models.User{
					UserId:         models.UserId(userIdString),
					OrganizationId: utils.TextToUUID(organizationId),
				},
			)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in another org", func(t *testing.T) {
			userIdString := "11111111-1111-1111-1111-111111111111"
			userIdParsed := uuid.MustParse(userIdString)
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{
					InboxId: adminTestInboxId,
					UserId:  userIdParsed,
				}, nil, models.Inbox{
					Id: adminTestInboxId, OrganizationId: anotherOrgIdUUID,
				}, models.User{
					UserId:         models.UserId(userIdString),
					OrganizationId: utils.TextToUUID(organizationId),
				},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})
	})

	t.Run("non admin", func(t *testing.T) {
		organizationId_nonadmin := "orgId"
		anotherOrganizationId_nonadmin := "anotherOrgId"
		actorIdentity_nonadmin := models.Identity{
			UserId: models.UserId("00000000-0000-0000-0000-000000000000"),
		}

		// Use distinct UUIDs for target inbox vs actor's own inboxes for negative tests
		targetInboxId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		actorsOwnInboxId := uuid.MustParse("22222222-2222-2222-2222-222222222222")

		creds := models.Credentials{
			Role:           models.BUILDER,
			OrganizationId: utils.TextToUUID(organizationId_nonadmin), ActorIdentity: actorIdentity_nonadmin,
		}
		orgIdUUIDString_nonadmin := creds.OrganizationId
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to create an inbox user in an inbox if the calling user is admin of the inbox", func(t *testing.T) {
			actorMemberUserId := uuid.MustParse("00000000-0000-0000-0000-000000000000")
			createInputTargetUserId := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{
					InboxId: targetInboxId,
					UserId:  createInputTargetUserId,
				}, // Attempting to create in targetInboxId
				[]models.InboxUser{{
					InboxId: targetInboxId,
					Role:    models.InboxUserRoleAdmin, UserId: actorMemberUserId,
				}}, // Actor is admin of targetInboxId
				models.Inbox{Id: targetInboxId, OrganizationId: orgIdUUIDString_nonadmin},
				models.User{
					UserId:         models.UserId(createInputTargetUserId.String()),
					OrganizationId: utils.TextToUUID(organizationId_nonadmin),
				},
			)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in an inbox if the calling user is not member of the inbox", func(t *testing.T) {
			actorMemberUserId := uuid.MustParse("00000000-0000-0000-0000-000000000000")
			createInputTargetUserId := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{
					InboxId: targetInboxId,
					UserId:  createInputTargetUserId,
				}, // Attempting to create in targetInboxId
				[]models.InboxUser{{
					InboxId: actorsOwnInboxId,
					Role:    models.InboxUserRoleAdmin, UserId: actorMemberUserId,
				}}, // Actor is admin of a *different* inbox
				models.Inbox{Id: targetInboxId, OrganizationId: orgIdUUIDString_nonadmin},
				models.User{
					UserId:         models.UserId(createInputTargetUserId.String()),
					OrganizationId: utils.TextToUUID(organizationId_nonadmin),
				},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in an inbox if the calling user is not admin of the inbox", func(t *testing.T) {
			actorMemberUserId := uuid.MustParse("00000000-0000-0000-0000-000000000000")
			createInputTargetUserId := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{
					InboxId: targetInboxId,
					UserId:  createInputTargetUserId,
				}, // Attempting to create in targetInboxId
				[]models.InboxUser{{
					InboxId: targetInboxId,
					Role:    models.InboxUserRoleMember, UserId: actorMemberUserId,
				}}, // Actor is only a member, not admin
				models.Inbox{Id: targetInboxId, OrganizationId: orgIdUUIDString_nonadmin},
				models.User{
					UserId:         models.UserId(createInputTargetUserId.String()),
					OrganizationId: utils.TextToUUID(organizationId_nonadmin),
				},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user if the target user does not belong to the right org", func(t *testing.T) {
			actorMemberUserId := uuid.MustParse("00000000-0000-0000-0000-000000000000")
			createInputTargetUserId := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: targetInboxId, UserId: createInputTargetUserId},
				[]models.InboxUser{{
					InboxId: targetInboxId,
					Role:    models.InboxUserRoleAdmin, UserId: actorMemberUserId,
				}}, // Actor is admin of the target inbox
				models.Inbox{Id: targetInboxId, OrganizationId: orgIdUUIDString_nonadmin},
				models.User{
					UserId:         models.UserId(createInputTargetUserId.String()),
					OrganizationId: utils.TextToUUID(anotherOrganizationId_nonadmin),
				}, // Target user in different org
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})
	})
}
