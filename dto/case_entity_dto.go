package dto

import "github.com/checkmarble/marble-backend/models"

type CaseEntityDto struct {
	TableName string                    `json:"table_name"`
	ObjectId  string                    `json:"object_id"`
	Sources   []models.CaseEntitySource `json:"sources"`
	Data      map[string]any            `json:"data"`
}

func adaptCaseEntities(entities []models.CaseEntity) []CaseEntityDto {
	result := make([]CaseEntityDto, 0, len(entities))
	for _, entity := range entities {
		result = append(result, CaseEntityDto{TableName: entity.TableName, ObjectId: entity.ObjectId, Sources: entity.Sources, Data: adaptClientObjectData(entity.Data)})
	}
	return result
}

type CaseEntityRefBody struct {
	TableName string `json:"table_name"`
	ObjectId  string `json:"object_id"`
}
type UpdateCaseEntitiesBody struct {
	Entities []CaseEntityRefBody `json:"entities"`
}

func AdaptCaseEntityRefs(refs []CaseEntityRefBody) []models.CaseEntityRef {
	result := make([]models.CaseEntityRef, len(refs))
	for i, ref := range refs {
		result[i] = models.CaseEntityRef{TableName: ref.TableName, ObjectId: ref.ObjectId}
	}
	return result
}
