package dto

import (
	"encoding/json"
	"github.com/checkmarble/marble-backend/models"
	"maps"
)

type CaseEntityDto struct {
	TableName string                    `json:"table_name"`
	ObjectId  string                    `json:"object_id"`
	Sources   []models.CaseEntitySource `json:"sources"`
	Data      map[string]any            `json:"-"`
}

func (entity CaseEntityDto) MarshalJSON() ([]byte, error) {
	var data json.RawMessage
	if entity.Data != nil {
		// Reuse the established coordinate and metadata-key conversion without mutating model data.
		detail, err := json.Marshal(ClientObjectDetail{Data: maps.Clone(entity.Data)})
		if err != nil {
			return nil, err
		}
		var fields struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(detail, &fields); err != nil {
			return nil, err
		}
		data = fields.Data
	}
	return json.Marshal(struct {
		TableName string                    `json:"table_name"`
		ObjectId  string                    `json:"object_id"`
		Sources   []models.CaseEntitySource `json:"sources"`
		Data      json.RawMessage           `json:"data"`
	}{entity.TableName, entity.ObjectId, entity.Sources, data})
}
func adaptCaseEntities(entities []models.CaseEntity) []CaseEntityDto {
	result := make([]CaseEntityDto, 0, len(entities))
	for _, entity := range entities {
		result = append(result, CaseEntityDto{TableName: entity.TableName, ObjectId: entity.ObjectId, Sources: entity.Sources, Data: entity.Data})
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
