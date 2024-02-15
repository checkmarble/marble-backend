package pg_indexes

import (
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestParseCreateIndexStatement(t *testing.T) {
	t.Run("With 2 indexed and 4 included and WHERE condition", func(t *testing.T) {
		asserts := assert.New(t)
		idx := parseCreateIndexStatement(
			`CREATE INDEX index_name ON "org-test"."Transaction" USING btree (userpublicid, eventdate DESC) INCLUDE (object_id, transactionamount, operationdirection, operationcode) WHERE (valid_until = 'infinity'::timestamp with time zone)`,
		)
		asserts.Equal(
			models.ConcreteIndex{
				Indexed:  []models.FieldName{"userpublicid", "eventdate"},
				Included: []models.FieldName{"object_id", "transactionamount", "operationdirection", "operationcode"},
			},
			idx,
		)
	})

	t.Run("With 2 indexed and 4 included", func(t *testing.T) {
		asserts := assert.New(t)
		idx := parseCreateIndexStatement(
			`CREATE INDEX index_name ON "org-test"."Transaction" USING btree (userpublicid, eventdate DESC) INCLUDE (object_id, transactionamount, operationdirection, operationcode)`,
		)
		asserts.Equal(
			models.ConcreteIndex{
				Indexed:  []models.FieldName{"userpublicid", "eventdate"},
				Included: []models.FieldName{"object_id", "transactionamount", "operationdirection", "operationcode"},
			},
			idx,
		)
	})

	t.Run("With 2 indexed", func(t *testing.T) {
		asserts := assert.New(t)
		idx := parseCreateIndexStatement(`CREATE INDEX index_name ON Transaction (userpublicid DESC)`)
		fmt.Println(idx)
		asserts.Equal(
			models.ConcreteIndex{
				Indexed:  []models.FieldName{"userpublicid"},
				Included: nil,
			},
			idx,
		)
	})
}
