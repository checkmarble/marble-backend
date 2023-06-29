package server

import (
	"context"
	"log"
	"marble/marble-backend/infra"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/repositories"
	"marble/marble-backend/server/api"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slog"
)

type Server struct {
	Config               Config
	pgRepository         *pg_repository.PGRepository
	marbleConnectionPool *pgxpool.Pool
	logger               *slog.Logger
}

func NewServer(config Config, logger *slog.Logger) (s *Server, err error) {
	// The below specifically does not share a connection pool with the functions "run migrations" and "wipe db" because it conflicts
	// with the postgresql search path update
	connectionString := config.PGConfig.GetConnectionString(config.Env)
	marbleConnectionPool, err := infra.NewPostgresConnectionPool(connectionString)
	if err != nil {
		log.Fatal("error creating postgres connection to marble database", err.Error())
	}

	pgRepository, err := pg_repository.New(marbleConnectionPool)
	if err != nil {
		logger.Error("error creating pg repository:\n", err.Error())
		return
	}
	s = &Server{
		Config:               config,
		pgRepository:         pgRepository,
		marbleConnectionPool: marbleConnectionPool,
		logger:               logger,
	}
	return
}

func (s *Server) Run() {
	ctx := context.Background()

	devEnv := s.Config.Env == "DEV"

	corsAllowLocalhost := devEnv

	marbleJwtSigningKey := infra.MustParseSigningKey(utils.GetRequiredStringEnv("AUTHENTICATION_JWT_SIGNING_KEY"))

	repositories, err := repositories.NewRepositories(
		s.Config.GlobalConfiguration,
		marbleJwtSigningKey,
		infra.IntializeFirebase(ctx),
		s.pgRepository,
		s.marbleConnectionPool,
		s.logger,
	)
	if err != nil {
		panic(err)
	}

	usecases := usecases.Usecases{
		Repositories:  *repositories,
		Configuration: s.Config.GlobalConfiguration,
	}

	////////////////////////////////////////////////////////////
	// Seed the database
	////////////////////////////////////////////////////////////

	seedUsecase := usecases.NewSeedUseCase()

	marbleAdminEmail, _ := os.LookupEnv("MARBLE_ADMIN_EMAIL")
	if marbleAdminEmail != "" {
		err := seedUsecase.SeedMarbleAdmins(marbleAdminEmail)
		if err != nil {
			panic(err)
		}
	}

	if devEnv {
		zorgOrganizationId := "13617a88-56f5-4baa-8d11-ce102f7da907"
		err := seedUsecase.SeedZorgOrganization(zorgOrganizationId)
		if err != nil {
			panic(err)
		}
	}

	api, _ := api.New(ctx, s.Config.Port, usecases, s.logger, corsAllowLocalhost)

	////////////////////////////////////////////////////////////
	// Start serving the app
	////////////////////////////////////////////////////////////

	// Intercept SIGxxx signals
	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting server on port %v\n", s.Config.Port)
		if err := api.ListenAndServe(); err != nil {
			s.logger.ErrorCtx(ctx, "error serving the app: \n"+err.Error())
		}
		s.logger.InfoCtx(ctx, "server returned")
	}()

	// Block until we receive our signal.
	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	api.Shutdown(shutdownCtx)
}
