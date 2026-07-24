package integration

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/twpayne/go-geom"
)

func TestGeomFromPostgres(t *testing.T) {
	rows, err := pgPool.Query(t.Context(), `select st_geomfromtext('point(-73.935242 40.730610)', 4326)`)

	assert.NoError(t, err)

	var loc models.Location

	for rows.Next() {
		assert.NoError(t, rows.Scan(&loc))

		assert.Equal(t, -73.935242, loc.X())
		assert.Equal(t, 40.730610, loc.Y())
	}

	assert.NoError(t, rows.Err())
}

func TestGeomToPostgres(t *testing.T) {
	loc := models.Location{
		Point: geom.NewPointFlat(geom.XY, []float64{-74.935242, 40.730610}),
	}

	rows, err := pgPool.Query(t.Context(), `select $1::geometry = st_geomfromtext('point(-74.935242 40.730610)', 4326)`, loc)

	assert.NoError(t, err)

	var b bool

	for rows.Next() {
		assert.NoError(t, rows.Scan(&b))
		assert.True(t, b)
	}

	assert.NoError(t, rows.Err())
}
