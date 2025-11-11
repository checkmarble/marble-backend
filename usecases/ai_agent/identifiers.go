package ai_agent

import (
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

// TMP, to be moved to a common place if we keep it here.
func getLinkedDatabaseIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Node, error) {
	dataAccessors := []ast.Node{}
	var recursiveDatabaseAccessor func(
		baseTable string,
		path []string,
		links map[string]models.LinkToSingle,
		visited []string,
	) error

	triggerObjectTable, found := dataModel.Tables[scenario.TriggerObjectType]
	if !found {
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}

	recursiveDatabaseAccessor = func(
		baseTable string,
		path []string,
		links map[string]models.LinkToSingle,
		visited []string,
	) error {
		visited = append(visited, baseTable)
		for linkName, link := range links {
			table, found := dataModel.Tables[link.ParentTableName]
			if !found {
				return fmt.Errorf("table %s not found in data model", scenario.TriggerObjectType)
			}
			if slices.Contains(visited, table.Name) {
				continue
			}
			visitedDeepCp := append(make([]string, 0, len(visited)+1), visited...)
			visitedDeepCp = append(visitedDeepCp, table.Name)

			// deepcopy so that different identifiers don't collide
			pathForLink := append(make([]string, 0, len(path)+1), path...)
			pathForLink = append(pathForLink, linkName)

			for fieldName := range table.Fields {
				dataAccessors = append(dataAccessors,
					ast.NewNodeDatabaseAccess(
						scenario.TriggerObjectType,
						fieldName,
						pathForLink,
					),
				)
			}

			if err := recursiveDatabaseAccessor(
				table.Name, pathForLink,
				table.LinksToSingle, visitedDeepCp); err != nil {
				return err
			}
		}
		return nil
	}

	if err := recursiveDatabaseAccessor(
		triggerObjectTable.Name,
		[]string{},
		triggerObjectTable.LinksToSingle,
		[]string{},
	); err != nil {
		return nil, err
	}
	return dataAccessors, nil
}

func getPayloadIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Node, error) {
	dataAccessors := []ast.Node{}

	triggerObjectTable, found := dataModel.Tables[scenario.TriggerObjectType]
	if !found {
		// unexpected error: must be a valid table
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}
	for fieldName := range triggerObjectTable.Fields {
		dataAccessors = append(dataAccessors,
			ast.Node{
				Function: ast.FUNC_PAYLOAD,
				Constant: nil,
				Children: []ast.Node{
					ast.NewNodeConstant(fieldName),
				},
			},
		)
	}
	return dataAccessors, nil
}
