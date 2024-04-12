package pg_indexes

import (
	"regexp"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

var columnNamesRegex = regexp.MustCompile(`(\([a-zA-Z0-9,_\ ]+\))`)

type PGIndex struct {
	CreationInProgress bool
	Definition         string
	IsValid            bool
	IsUnique           bool
	Name               string
	RelationId         uint32
	TableName          string
}

func parseCreateIndexStatement(sql string) models.ConcreteIndex {
	// This function EXPECTS the input to be a correctly formatted CREATE INDEX statement
	// coming straight from the DB. It will not work correctly if it is not one.
	// However, we do not try to parse it all generality because that is beyond the scope of this function.
	// Moreover, column names are enforced to be alphanumeric + "_" only at data model edition time, so
	// we do not expect column names to be double quoted, to include special characters other than "_", etc.
	// We also expect concrete column names only indexed (not expressions, etc.)
	matches := columnNamesRegex.FindAllString(sql, -1)

	// len(matches) is expected to be > 0 because the sql statement is expected to be a proper CREATE INDEX statement
	indexedColumnsRaw := strings.Split(strings.Trim(matches[0], "() "), ",")
	indexedColumnNames := pure_utils.Map(indexedColumnsRaw, func(s string) string {
		// We discard the order of the index (ASC/DESC) because this is not relevant or modelized (yet) for our purposes
		parts := strings.Split(strings.Trim(s, " "), " ")
		// the first part of the string must be the column name
		return parts[0]
	})

	var includedColumnNames []string
	// if there is a second match, it is the list of included columns (optional)
	if len(matches) > 1 {
		names := strings.Split(strings.Trim(matches[1], "() "), ",")
		includedColumnNames = pure_utils.Map(names, func(s string) string {
			return strings.Trim(s, " ")
		})
	}

	return models.ConcreteIndex{
		Indexed:  indexedColumnNames,
		Included: includedColumnNames,
	}
}

func (pgIndex PGIndex) AdaptConcreteIndex() models.ConcreteIndex {
	idx := parseCreateIndexStatement(pgIndex.Definition)

	idx.TableName = models.TableName(pgIndex.TableName)
	return idx
}

func (pgIndex PGIndex) AdaptUnicityIndex() (bool, models.UnicityIndex) {
	if !pgIndex.IsUnique {
		return false, models.UnicityIndex{}
	}

	idx := pgIndex.AdaptConcreteIndex()

	out := models.UnicityIndex{
		Fields:    idx.Indexed,
		TableName: idx.TableName,
	}
	out.CreationInProcess = pgIndex.CreationInProgress
	return true, out
}
