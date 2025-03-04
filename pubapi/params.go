package pubapi

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func UuidParam(c *gin.Context, param string) (*uuid.UUID, error) {
	parsed, err := uuid.Parse(c.Param(param))
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
