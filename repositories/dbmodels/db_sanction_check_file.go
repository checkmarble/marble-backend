package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBSanctionCheckFile struct {
	Id              string    `db:"id"`
	SanctionCheckId string    `db:"sanction_check_id"`
	BucketName      string    `db:"bucket_name"`
	FileReference   string    `db:"file_reference"`
	FileName        string    `db:"file_name"`
	CreatedAt       time.Time `db:"created_at"`
}

const TABLE_SANCTION_CHECK_FILES = "sanction_check_files"

var SelectSanctionCheckFileColumn = utils.ColumnList[DBSanctionCheckFile]()

func AdaptSanctionCheckFile(db DBSanctionCheckFile) (models.SanctionCheckFile, error) {
	return models.SanctionCheckFile{
		Id:              db.Id,
		SanctionCheckId: db.SanctionCheckId,
		CreatedAt:       db.CreatedAt,
		BucketName:      db.BucketName,
		FileName:        db.FileName,
		FileReference:   db.FileReference,
	}, nil
}
