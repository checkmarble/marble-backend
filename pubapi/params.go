package pubapi

import (
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func UuidParam(c *gin.Context, param string) (*uuid.UUID, error) {
	parsed, err := uuid.Parse(c.Param(param))
	if err != nil {
		return nil, errors.WithDetail(ErrInvalidPayload, err.Error())
	}

	return &parsed, nil
}
