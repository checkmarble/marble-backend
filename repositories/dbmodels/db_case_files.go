package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBCaseFile struct {
	Id            string    `db:"id"`
	CreatedAt     time.Time `db:"created_at"`
	CaseId        string    `db:"case_id"`
	BucketName    string    `db:"bucket_name"`
	FileReference string    `db:"file_reference"`
	FileName      string    `db:"file_name"`
}

const TABLE_CASE_FILES = "case_files"

var SelectCaseFileColumn = utils.ColumnList[DBCaseFile]()

func AdaptCaseFile(db DBCaseFile) (models.CaseFile, error) {
	return models.CaseFile{
		Id:            db.Id,
		CaseId:        db.CaseId,
		CreatedAt:     db.CreatedAt,
		BucketName:    db.BucketName,
		FileName:      db.FileName,
		FileReference: db.FileReference,
	}, nil
}
