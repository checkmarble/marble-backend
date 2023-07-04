package organization

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type OrganizationCreator struct {
	TransactionFactory         repositories.TransactionFactory
	OrganizationRepository     repositories.OrganizationRepository
	DataModelRepository        repositories.DataModelRepository
	OrganizationSeeder         OrganizationSeeder
	PopulateOrganizationSchema PopulateOrganizationSchema
	Repositories               repositories.Repositories
}

func (creator *OrganizationCreator) CreateOrganizationWithId(newOrganizationId string, createOrga models.CreateOrganizationInput) (models.Organization, error) {

	organization, err := repositories.TransactionReturnValue(creator.TransactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Organization, error) {
		if err := creator.OrganizationRepository.CreateOrganization(tx, createOrga, newOrganizationId); err != nil {
			return models.Organization{}, err
		}
		if err := creator.createDataModel(tx, newOrganizationId); err != nil {
			return models.Organization{}, err
		}
		return creator.OrganizationRepository.GetOrganizationById(tx, newOrganizationId)
	})

	if err != nil {
		return models.Organization{}, err
	}

	err = creator.OrganizationSeeder.Seed(organization.ID)
	if err != nil {
		return models.Organization{}, err
	}

	_, err = repositories.TransactionReturnValue(creator.TransactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (any, error) {
		// store client's data in marble DB
		orgDatabase := models.DATABASE_MARBLE
		err := creator.PopulateOrganizationSchema.CreateOrganizationSchema(tx, organization, orgDatabase)

		return nil, err
	})
	if err != nil {
		return models.Organization{}, err
	}

	return organization, nil
}

func (creator *OrganizationCreator) createDataModel(tx repositories.Transaction, organizationId string) error {

	///////////////////////////////
	// Create and store a data model
	///////////////////////////////
	return creator.DataModelRepository.CreateDataModel(tx, organizationId, models.DataModel{
		Tables: map[models.TableName]models.Table{
			"transactions": {
				Name: "transactions",
				Fields: map[models.FieldName]models.Field{
					"object_id": {
						DataType: models.String,
					},
					"updated_at":  {DataType: models.Timestamp},
					"account_id":  {DataType: models.String, Nullable: true},
					"bic_country": {DataType: models.String, Nullable: true},
					"country":     {DataType: models.String, Nullable: true},
					"description": {DataType: models.String, Nullable: true},
					"direction":   {DataType: models.String, Nullable: true},
					"status":      {DataType: models.String, Nullable: true},
					"title":       {DataType: models.String, Nullable: true},
					"amount":      {DataType: models.Float, Nullable: true},
				},
				LinksToSingle: map[models.LinkName]models.LinkToSingle{
					"account": {
						LinkedTableName: "accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "account_id"},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[models.FieldName]models.Field{
					"object_id": {
						DataType: models.String,
					},
					"updated_at": {DataType: models.Timestamp},
					"balance":    {DataType: models.Float, Nullable: true},
					"company_id": {DataType: models.String, Nullable: true},
					"name":       {DataType: models.String, Nullable: true},
					"currency":   {DataType: models.String, Nullable: true},
					"is_frozen":  {DataType: models.Bool},
				},
				LinksToSingle: map[models.LinkName]models.LinkToSingle{
					"company": {
						LinkedTableName: "companies",
						ParentFieldName: "object_id",
						ChildFieldName:  "company_id"},
				},
			},
			"companies": {
				Name: "companies",
				Fields: map[models.FieldName]models.Field{
					"object_id": {
						DataType: models.String,
					},
					"updated_at": {DataType: models.Timestamp},
					"name":       {DataType: models.String, Nullable: true},
				},
			},
		},
	})
}
