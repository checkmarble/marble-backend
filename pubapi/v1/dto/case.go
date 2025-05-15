package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type Case struct {
	Id string `json:"id"`
}

func AdaptCase(c models.Case) Case {
	return Case{
		Id: c.Id,
	}
}
