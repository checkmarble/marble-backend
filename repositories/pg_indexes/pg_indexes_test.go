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
				Indexed:  []string{"userpublicid", "eventdate"},
				Included: []string{"object_id", "transactionamount", "operationdirection", "operationcode"},
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
				Indexed:  []string{"userpublicid", "eventdate"},
				Included: []string{"object_id", "transactionamount", "operationdirection", "operationcode"},
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
				Indexed:  []string{"userpublicid"},
				Included: nil,
			},
			idx,
		)
	})

	t.Run("With escaped field name", func(t *testing.T) {
		asserts := assert.New(t)
		idx := parseCreateIndexStatement(`CREATE INDEX index_name ON Transaction (id, "userpublicid" DESC)`)
		fmt.Println(idx)
		asserts.Equal(
			models.ConcreteIndex{
				Indexed:  []string{"id", "userpublicid"},
				Included: nil,
			},
			idx,
		)
	})

	t.Run("With escaped field name, case sensitive", func(t *testing.T) {
		asserts := assert.New(t)
		idx := parseCreateIndexStatement(`CREATE INDEX index_name ON Transaction (id, "TIME" DESC)`)
		fmt.Println(idx)
		asserts.Equal(
			models.ConcreteIndex{
				Indexed:  []string{"id", "TIME"},
				Included: nil,
			},
			idx,
		)
	})

	t.Run("With escaped field name in included", func(t *testing.T) {
		asserts := assert.New(t)
		idx := parseCreateIndexStatement(`CREATE INDEX index_name ON Transaction (id, "userpublicid" DESC) INCLUDE ("time")`)
		fmt.Println(idx)
		asserts.Equal(
			models.ConcreteIndex{
				Indexed:  []string{"id", "userpublicid"},
				Included: []string{"time"},
			},
			idx,
		)
	})
}
