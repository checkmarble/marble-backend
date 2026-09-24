package dbmodels

import (
	"bytes"
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type DbRoleBinding struct {
	Id                uuid.UUID       `db:"id"`
	OrgId             *uuid.UUID      `db:"org_id"`
	UserId            *uuid.UUID      `db:"user_id"`
	ApiKeyId          *uuid.UUID      `db:"api_key_id"`
	NativeRole        *string         `db:"native_role"`
	CustomRoleId      *uuid.UUID      `db:"custom_role_id"`
	Conditions        json.RawMessage `db:"conditions"`
	CustomRoleSlug    *string         `db:"custom_role_slug"`
	CustomPermissions []string        `db:"custom_permissions"`
}

func AdaptRoleBinding(db DbRoleBinding) (models.RoleBinding, error) {
	conditions := models.RoleBindingConditions{}

	decoder := json.NewDecoder(bytes.NewReader(db.Conditions))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&conditions); err != nil {
		return models.RoleBinding{}, err
	}
	if err := conditions.Validate(); err != nil {
		return models.RoleBinding{}, err
	}

	binding := models.RoleBinding{
		Id:           db.Id,
		ApiKeyId:     db.ApiKeyId,
		CustomRoleId: db.CustomRoleId,
		Conditions:   conditions,
	}

	if db.OrgId != nil {
		binding.OrgId = *db.OrgId
	}

	if db.UserId != nil {
		userId := models.UserId(db.UserId.String())
		binding.UserId = &userId
	}

	if db.NativeRole != nil {
		binding.Role = models.Role(*db.NativeRole)
		binding.Permissions = binding.Role.Permissions()
	}

	if db.CustomRoleId != nil && db.CustomRoleSlug != nil {
		binding.Role = models.Role(*db.CustomRoleSlug)
		binding.Permissions = pure_utils.Map(db.CustomPermissions, func(permission string) models.Permission {
			return models.Permission(permission)
		})
	}

	return binding, nil
}

func AdaptRoleBindingModel(db DbRoleBinding) (models.RoleBinding, error) {
	return AdaptRoleBinding(db)
}
