package dbmodels

import "github.com/checkmarble/marble-backend/models"

type DbAssignableUserWithCaseCount struct {
	DBUserResult

	CaseCount int `db:"case_count"`
}

func AdaptAssignableUserWithCaseCount(db DbAssignableUserWithCaseCount) (models.UserWithCaseCount, error) {
	user, err := AdaptUser(db.DBUserResult)

	if err != nil {
		return models.UserWithCaseCount{}, err
	}

	return models.UserWithCaseCount{
		User:      user,
		CaseCount: db.CaseCount,
	}, nil
}
