package dto

import "github.com/checkmarble/marble-backend/models"

type Client360Table struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Alias       string `json:"alias"`
	Description string `json:"description"`
	Ready       bool   `json:"ready"`
}

func AdaptClient360Table(table models.Client360Table) Client360Table {
	return Client360Table{
		Id:          table.ID,
		Name:        table.Name,
		Alias:       table.Alias,
		Description: table.Description,
		Ready:       table.IndexReady,
	}
}

type Client360SearchInput struct {
	Table string `json:"table"`
	Terms string `json:"terms"`
}
