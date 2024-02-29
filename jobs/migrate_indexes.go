package jobs

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func generateUsecaseWithCredForMarbleAdmin(ctx context.Context, jobUsecases usecases.Usecases, orgId string) usecases.UsecasesWithCreds {
	creds := models.Credentials{
		Role:           models.MARBLE_ADMIN,
		OrganizationId: orgId,
	}
	return usecases.UsecasesWithCreds{
		Usecases:                jobUsecases,
		Credentials:             creds,
		Logger:                  utils.LoggerFromContext(ctx),
		OrganizationIdOfContext: func() (string, error) { return orgId, nil },
		Context:                 ctx,
	}
}

func MigrateIndexes(ctx context.Context, usecases usecases.Usecases) error {
	logger := utils.LoggerFromContext(ctx)

	exec := usecases.NewExecutorFactory().NewExecutor()
	organizations, err := usecases.Repositories.OrganizationRepository.AllOrganizations(ctx, exec)
	if err != nil {
		return err
	}

	for _, org := range organizations {
		usecasesWithCreds := generateUsecaseWithCredForMarbleAdmin(ctx, usecases, org.Id)
		dmUsecase := usecasesWithCreds.NewDataModelUseCase()
		var dataModel models.DataModel
		var err error
		dataModel, err = dmUsecase.GetDataModel(ctx, org.Id)
		if err != nil {
			logger.ErrorContext(ctx, "Error getting data model for org "+org.Id, err)
			return err
		}
		// iterate over tables
		for _, table := range dataModel.Tables {
			// 	start with the object_id field which needs to be unique in any case
			field := table.Fields["object_id"]
			if field.UnicityConstraint == models.NoUnicityConstraint {
				fmt.Println()
				logger.InfoContext(ctx, fmt.Sprintf("Adding index on object_id for table %s", string(table.Name)))
				done, err := dmUsecase.AddUniqueIndex(
					ctx,
					org.Id,
					table.Name,
					field.Name,
				)
				if err != nil {
					logger.ErrorContext(ctx, "Error adding index on object_id for table "+string(table.Name), err)
					return err
				} else if done {
					logger.InfoContext(ctx, "Added index on object_id for table "+string(table.Name))
				}
				// refresh data model so that the field.UnicityConstraint is up to date for the next iteration
				dataModel, err = dmUsecase.GetDataModel(ctx, org.Id)
				if err != nil {
					logger.ErrorContext(ctx, "Error getting data model for org "+org.Id, err)
					return err
				}
			}

		}
		// iterate over tables again to treat link targets
		for _, table := range dataModel.Tables {
			// iterate over links to single and check the parent table fields
			for _, link := range table.LinksToSingle {
				parentTable := dataModel.Tables[link.LinkedTableName]
				field := parentTable.Fields[link.ParentFieldName]
				if field.UnicityConstraint == models.NoUnicityConstraint {
					fmt.Println()
					logger.InfoContext(ctx, fmt.Sprintf("Adding index on link target %s for table %s",
						string(field.Name), string(parentTable.Name)))
					done, err := dmUsecase.AddUniqueIndex(
						ctx,
						org.Id,
						parentTable.Name,
						field.Name,
					)
					if err != nil {
						logger.ErrorContext(ctx, "Error adding index on object_id for table "+string(parentTable.Name), err)
						return err
					} else if done {
						logger.InfoContext(ctx, "Added index on "+
							string(field.Name)+" for table "+string(parentTable.Name))
					}
					// refresh data model so that the field.UnicityConstraint is up to date for the next iteration
					dataModel, err = dmUsecase.GetDataModel(ctx, org.Id)
					if err != nil {
						logger.ErrorContext(ctx, "Error getting data model for org "+org.Id, err)
						return err
					}
				}
			}
		}
	}

	return nil
}
