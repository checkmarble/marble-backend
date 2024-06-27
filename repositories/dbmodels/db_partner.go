package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBPartner struct {
	Id        string    `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	Name      string    `db:"name"`
	Bic       string    `db:"bic"`
}

const TABLE_PARTNERS = "partners"

var PartnerFields = utils.ColumnList[DBPartner]()

func AdaptPartner(db DBPartner) (models.Partner, error) {
	return models.Partner{
		Id:        db.Id,
		CreatedAt: db.CreatedAt,
		Name:      db.Name,
		Bic:       db.Bic,
	}, nil
}
