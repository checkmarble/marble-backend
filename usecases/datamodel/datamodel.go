package datamodel

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
)

type dataModelRepository interface {
	GetDataModel(ctx context.Context, organizationID string, fetchEnumValues bool) (models.DataModel, error)
	CreateDataModelTable(ctx context.Context, organizationID, name, description string, defaultFields []models.DataModelField) (string, error)
	UpdateDataModelTable(ctx context.Context, tableID, description string) error
	CreateDataModelField(ctx context.Context, organizationID, tableID string, field models.DataModelField) (string, error)
	UpdateDataModelField(ctx context.Context, fieldID, description string) error
	CreateDataModelLink(ctx context.Context, link models.DataModelLink) error
	DeleteDataModel(ctx context.Context, organizationID string) error
}

type UseCase struct {
	repository dataModelRepository
}

func (u *UseCase) CreateTable(ctx context.Context, organizationID, name, description string) (string, error) {
	defaultFields := []models.DataModelField{
		{
			Name:        "object_id",
			Description: fmt.Sprintf("required id on all objects in the %s table", name),
			Type:        models.String.String(),
		},
		{
			Name:        "updated_at",
			Description: fmt.Sprintf("required timestamp on all objects in the %s table", name),
			Type:        models.Timestamp.String(),
		},
	}

	tableID, err := u.repository.CreateDataModelTable(ctx, organizationID, name, description, defaultFields)
	if err != nil {
		return "", fmt.Errorf("repository.CreateDataModelTable error: %w", err)
	}
	return tableID, nil
}

func (u *UseCase) UpdateDataModelTable(ctx context.Context, tableID, description string) error {
	err := u.repository.UpdateDataModelTable(ctx, tableID, description)
	if err != nil {
		return fmt.Errorf("repository.UpdateDataModelTable error: %w", err)
	}
	return nil
}

func (u *UseCase) CreateField(ctx context.Context, organizationID, tableID string, field models.DataModelField) (string, error) {
	fieldID, err := u.repository.CreateDataModelField(ctx, organizationID, tableID, field)
	if err != nil {
		return "", fmt.Errorf("repository.CreateDataModelField error: %w", err)
	}
	return fieldID, nil
}

func (u *UseCase) UpdateDataModelField(ctx context.Context, fieldID, description string) error {
	err := u.repository.UpdateDataModelField(ctx, fieldID, description)
	if err != nil {
		return fmt.Errorf("repository.UpdateDataModelField error: %w", err)
	}
	return nil
}

func (u *UseCase) CreateDataModelLink(ctx context.Context, link models.DataModelLink) error {
	err := u.repository.CreateDataModelLink(ctx, link)
	if err != nil {
		return fmt.Errorf("repository.CreateDataModelLink error: %w", err)
	}
	return nil
}

func (u *UseCase) GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error) {
	dataModel, err := u.repository.GetDataModel(ctx, organizationID, true)
	if err != nil {
		return models.DataModel{}, fmt.Errorf("repository.GetDataModel error: %w", err)
	}
	return dataModel, nil
}

func (u *UseCase) DeleteDataModel(ctx context.Context, organizationID string) error {
	err := u.repository.DeleteDataModel(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("repository.DeleteDataModel error: %w", err)
	}
	return nil
}

func New(r dataModelRepository) *UseCase {
	return &UseCase{
		repository: r,
	}
}
