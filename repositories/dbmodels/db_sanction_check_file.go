package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBScreeningFile struct {
	Id            string    `db:"id"`
	ScreeningId   string    `db:"sanction_check_id"`
	BucketName    string    `db:"bucket_name"`
	FileReference string    `db:"file_reference"`
	FileName      string    `db:"file_name"`
	CreatedAt     time.Time `db:"created_at"`
}

const TABLE_SCREENING_FILES = "sanction_check_files"

var SelectScreeningFileColumn = utils.ColumnList[DBScreeningFile]()

func AdaptScreeningFile(db DBScreeningFile) (models.ScreeningFile, error) {
	return models.ScreeningFile{
		Id:            db.Id,
		ScreeningId:   db.ScreeningId,
		CreatedAt:     db.CreatedAt,
		BucketName:    db.BucketName,
		FileName:      db.FileName,
		FileReference: db.FileReference,
	}, nil
}
