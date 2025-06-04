package security_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
)

func Test_ReadInbox(t *testing.T) {
	organizationId := "orgId"
	anotherOrganizationId := "anotherOrgId"
	// userId := models.UserId("userId") // Unused variable, specific actorIdentities defined in subtests
	// actorIdentity := models.Identity{ // Unused variable
	// 	UserId: userId,
	// }
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
		// For this test case, actor's ID must be a string representation of a valid UUID
		// to match the UserId in InboxUsers after .String() conversion.
		actorUserIdString := "30000000-0000-0000-0000-000000000001"
		actorParsedUUID := uuid.MustParse(actorUserIdString)
		specificActorIdentity := models.Identity{UserId: models.UserId(actorUserIdString)}

		creds := models.Credentials{Role: models.BUILDER, OrganizationId: organizationId, ActorIdentity: specificActorIdentity}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("User is member of the inbox", func(t *testing.T) {
			// The InboxUser's UserId must be the UUID form of the actor's string ID
			err := sec.ReadInbox(models.Inbox{
				OrganizationId: organizationId,
				InboxUsers:     []models.InboxUser{{UserId: actorParsedUUID}},
			})
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
	userId := models.UserId("userId") // This remains string for ActorIdentity for some tests
	actorIdentity := models.Identity{
		UserId: userId,
	}
	// Use valid UUID strings for parsing to avoid uuid.Nil issues
	inboxIdString1 := "10000000-0000-0000-0000-000000000001"
	parsedInboxId1 := uuid.MustParse(inboxIdString1)

	inboxIdString2 := "10000000-0000-0000-0000-000000000002" // Different UUID for "anotherInboxId"
	parsedInboxId2 := uuid.MustParse(inboxIdString2)


	t.Run("admin", func(t *testing.T) {
		creds := models.Credentials{Role: models.ADMIN, OrganizationId: organizationId}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to read any inbox user from the org", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{InboxId: parsedInboxId1, OrganizationId: organizationId}, nil)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("Should not be able to read any inbox user from another org", func(t *testing.T) {
			err := sec.ReadInboxUser(models.InboxUser{
				InboxId:        parsedInboxId1, // Target user is in parsedInboxId1
				OrganizationId: anotherOrganizationId,
			}, nil)
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
			// Actor is part of parsedInboxId1, target user is also in parsedInboxId1
			err := sec.ReadInboxUser(models.InboxUser{InboxId: parsedInboxId1}, []models.InboxUser{{InboxId: parsedInboxId1}})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
		t.Run("Should not be able to read an inbox user if the calling user is not a member of the inbox", func(t *testing.T) {
			// Actor is part of parsedInboxId2, target user is in parsedInboxId1
			err := sec.ReadInboxUser(models.InboxUser{InboxId: parsedInboxId1}, []models.InboxUser{
				{InboxId: parsedInboxId2},
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
		// Define a general parsedInboxId for admin tests, distinct from Test_ReadInboxUser's
		adminTestInboxId := uuid.MustParse("c0000000-0000-0000-0000-000000000001")

		creds := models.Credentials{Role: models.ADMIN, OrganizationId: organizationId}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to create an inbox user in any inbox", func(t *testing.T) {
			createInputTargetUserId, _ := uuid.Parse("d0000000-0000-0000-0000-000000000001")
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: adminTestInboxId, UserId: createInputTargetUserId}, nil, models.Inbox{
					Id: adminTestInboxId, OrganizationId: organizationId,
				}, models.User{UserId: models.UserId(createInputTargetUserId.String()), OrganizationId: organizationId},
			)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in another org", func(t *testing.T) {
			createInputTargetUserId2, _ := uuid.Parse("d0000000-0000-0000-0000-000000000002")
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: adminTestInboxId, UserId: createInputTargetUserId2}, nil, models.Inbox{
					Id: adminTestInboxId, OrganizationId: anotherOrganizationId, // Inbox in another org
				}, models.User{UserId: models.UserId(createInputTargetUserId2.String()), OrganizationId: organizationId},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})
	})

	t.Run("non admin", func(t *testing.T) {
		organizationId_nonadmin := "orgId" // Renamed to avoid conflict if outer scope vars were intended
		anotherOrganizationId_nonadmin := "anotherOrgId"
		userId_nonadmin_str := "30000000-0000-0000-0000-000000000001" // Actor's ID must be valid UUID string for some checks
		actorIdentity_nonadmin := models.Identity{ UserId: models.UserId(userId_nonadmin_str) }

		// Use distinct UUIDs for target inbox vs actor's own inboxes for negative tests
		targetInboxId := uuid.MustParse("b0000000-0000-0000-0000-000000000001") // Inbox where action is attempted
		actorsOwnInboxId := uuid.MustParse("b0000000-0000-0000-0000-000000000002") // Actor's actual inbox

		creds := models.Credentials{Role: models.BUILDER, OrganizationId: organizationId_nonadmin, ActorIdentity: actorIdentity_nonadmin}
		sec := security.EnforceSecurityInboxes{
			EnforceSecurity: &security.EnforceSecurityImpl{Credentials: creds},
			Credentials:     creds,
		}

		t.Run("Should be able to create an inbox user in an inbox if the calling user is admin of the inbox", func(t *testing.T) {
			actorMemberUserId, _ := uuid.Parse("a0000000-0000-0000-0000-00000000000a") // UUID for InboxUser.UserId
			createInputTargetUserId, _ := uuid.Parse("c0000000-0000-0000-0000-00000000000c") // UUID for CreateInboxUserInput.UserId

			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: targetInboxId, UserId: createInputTargetUserId}, // Attempting to create in targetInboxId
				[]models.InboxUser{{InboxId: targetInboxId, Role: models.InboxUserRoleAdmin, UserId: actorMemberUserId}}, // Actor is admin of targetInboxId
				models.Inbox{Id: targetInboxId, OrganizationId: organizationId_nonadmin},
				models.User{UserId: models.UserId(createInputTargetUserId.String()), OrganizationId: organizationId_nonadmin},
			)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in an inbox if the calling user is not member of the inbox", func(t *testing.T) {
			createInputTargetUserId2, _ := uuid.Parse("c0000000-0000-0000-0000-00000000000d")
			actorMemberUserId2, _ := uuid.Parse("a0000000-0000-0000-0000-00000000000b")
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: targetInboxId, UserId: createInputTargetUserId2}, // Attempting to create in targetInboxId
				[]models.InboxUser{{InboxId: actorsOwnInboxId, Role: models.InboxUserRoleAdmin, UserId: actorMemberUserId2}}, // Actor is admin of a *different* inbox
				models.Inbox{Id: targetInboxId, OrganizationId: organizationId_nonadmin},
				models.User{UserId: models.UserId(createInputTargetUserId2.String()), OrganizationId: organizationId_nonadmin},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user in an inbox if the calling user is not admin of the inbox", func(t *testing.T) {
			createInputTargetUserId3, _ := uuid.Parse("c0000000-0000-0000-0000-00000000000e")
			actorMemberUserId3, _ := uuid.Parse("a0000000-0000-0000-0000-00000000000c")
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: targetInboxId, UserId: createInputTargetUserId3}, // Attempting to create in targetInboxId
				[]models.InboxUser{{InboxId: targetInboxId, Role: models.InboxUserRoleMember, UserId: actorMemberUserId3}}, // Actor is only a member, not admin
				models.Inbox{Id: targetInboxId, OrganizationId: organizationId_nonadmin},
				models.User{UserId: models.UserId(createInputTargetUserId3.String()), OrganizationId: organizationId_nonadmin},
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})

		t.Run("Should not be able to create an inbox user if the target user does not belong to the right org", func(t *testing.T) {
			createInputTargetUserId4, _ := uuid.Parse("c0000000-0000-0000-0000-00000000000f")
			actorMemberUserId4, _ := uuid.Parse("a0000000-0000-0000-0000-00000000000d")
			err := sec.CreateInboxUser(
				models.CreateInboxUserInput{InboxId: targetInboxId, UserId: createInputTargetUserId4},
				[]models.InboxUser{{InboxId: targetInboxId, Role: models.InboxUserRoleAdmin, UserId: actorMemberUserId4}}, // Actor is admin of the target inbox
				models.Inbox{Id: targetInboxId, OrganizationId: organizationId_nonadmin},
				models.User{UserId: models.UserId(createInputTargetUserId4.String()), OrganizationId: anotherOrganizationId_nonadmin}, // Target user in different org
			)
			if err == nil {
				t.Errorf("Expected an error, got %v", err)
			}
		})
	})
}
