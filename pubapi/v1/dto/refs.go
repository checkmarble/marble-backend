package dto

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
)

type Ref struct {
	Id   string `json:"id"`
	Name string `json:"name,omitempty"`
}

func AdaptIdRef(id string) Ref {
	return Ref{Id: id}
}

func AdaptInboxRef(i models.Inbox) Ref {
	return Ref{
		Id:   i.Id.String(),
		Name: i.Name,
	}
}

func AdaptUserRef(u models.User) Ref {
	return Ref{
		Id:   string(u.UserId),
		Name: fmt.Sprintf("%s %s", u.FirstName, u.LastName),
	}
}

func AdaptTagRef(t models.Tag) Ref {
	return Ref{
		Id:   t.Id,
		Name: t.Name,
	}
}
