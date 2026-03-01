package dto

import "github.com/checkmarble/marble-backend/models"

type Tag struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Target string `json:"target"`
}

func AdaptTag(t models.Tag) Tag {
	return Tag{
		Id:     t.Id,
		Name:   t.Name,
		Target: string(t.Target),
	}
}
