package middlewares

import (
	"marble/marble-backend/usecases"

	"golang.org/x/exp/slog"
)

type Middlewares struct {
	usecases usecases.Usecases
	logger   *slog.Logger
}

func NewMiddlewares(usecases usecases.Usecases, logger *slog.Logger) (*Middlewares) {
	return &Middlewares{
		usecases: usecases,
		logger:   logger,
	}
}