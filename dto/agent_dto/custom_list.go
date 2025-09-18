package agent_dto

import "github.com/checkmarble/marble-backend/models"

type CustomList struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func AdaptCustomListDto(customList models.CustomList) CustomList {
	return CustomList{
		Id:          customList.Id,
		Name:        customList.Name,
		Description: customList.Description,
	}
}
