package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab.com/marble5/marble-backend-are-poc/api"
	"gitlab.com/marble5/marble-backend-are-poc/app"
	"gitlab.com/marble5/marble-backend-are-poc/pg_repository"
)

var version = "local-dev"
var appName = "marble-backend-are-poc"

// embed migrations sql folder
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

func main() {
	////////////////////////////////////////////////////////////
	// Init
	////////////////////////////////////////////////////////////
	log.Printf("starting %s version %s", appName, version)

	// Read ENV variables for configuration

	// API
	// Port
	port, ok := os.LookupEnv("PORT")
	if !ok || port == "" {
		log.Fatalf("set PORT environment variable")
	}

	// Postgres
	PGHostname, ok := os.LookupEnv("PG_HOSTNAME")
	if !ok || PGHostname == "" {
		log.Fatalf("set PG_HOSTNAME environment variable")
	}

	PGPort, ok := os.LookupEnv("PG_PORT")
	if !ok || PGPort == "" {
		log.Fatalf("set PG_PORT environment variable")
	}

	PGUser, ok := os.LookupEnv("PG_USER")
	if !ok || PGUser == "" {
		log.Fatalf("set PG_USER environment variable")
	}

	PGPassword, ok := os.LookupEnv("PG_PASSWORD")
	if !ok || PGPassword == "" {
		log.Fatalf("set PG_PASSWORD environment variable")
	}

	// Output config for debug before starting
	log.Printf("Port: %v", port)

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	// Postgres repository
	pgRepository, _ := pg_repository.New(PGHostname, PGPort, PGUser, PGPassword, embedMigrations)

	// In-memory repository
	// repository, _ := repository.New()

	app, _ := app.New(pgRepository)
	api, _ := api.New("8080", app)

	////////////////////////////////////////////////////////////
	// Start serving the app
	////////////////////////////////////////////////////////////

	// Intercept SIGxxx signals
	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting server on port %v\n", port)
		if err := api.ListenAndServe(); err != nil {
			log.Println(fmt.Errorf("error serving the app: %w", err))
		}
		log.Println("server returned")
	}()

	// Block until we receive our signal.
	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	api.Shutdown(shutdownCtx)

	log.Printf("stopping %s version %s", appName, version)

	// //datamodel tests
	// dm := engine.DataModel{
	// 	Tables: map[string]engine.Table{
	// 		"tx": {
	// 			Fields: map[string]engine.Field{
	// 				"id": {
	// 					DataType: engine.String,
	// 				},
	// 				"amount": {
	// 					DataType: engine.Float,
	// 				},
	// 				"sender_id": {
	// 					DataType: engine.String,
	// 				},
	// 			},
	// 			LinksToSingle: map[string]engine.LinkToSingle{
	// 				"sender": {
	// 					LinkedTableName: "user",
	// 					ParentFieldName: "sender_id",
	// 					ChildFieldName:  "id",
	// 				},
	// 			},
	// 		},
	// 		"user": {
	// 			Fields: map[string]engine.Field{
	// 				"id": {
	// 					DataType: engine.String,
	// 				},
	// 				"name": {
	// 					DataType: engine.String,
	// 				},
	// 			},
	// 		},
	// 	},
	// }
	// spew.Dump(dm)

	// // Payload
	// p := engine.Payload{
	// 	TableName: "tx",
	// 	Data: map[string]interface{}{
	// 		"id":        "2713",
	// 		"amount":    5.0,
	// 		"sender_id": "42",
	// 	}}

	// spew.Dump(p)
	// // Expression tests
	// exprs := []engine.Node{

	// 	// Basic Logical & operators
	// 	engine.And{engine.True{}, engine.True{}},
	// 	engine.And{engine.True{}, engine.False{}},
	// 	engine.And{engine.True{}, engine.And{engine.True{}, engine.Eq{engine.IntValue{5}, engine.IntValue{5}}}},
	// 	engine.And{engine.True{}, engine.And{engine.True{}, engine.Eq{engine.IntValue{6}, engine.IntValue{5}}}},
	// 	engine.And{engine.True{}, engine.And{engine.True{}, engine.Eq{engine.FloatValue{5}, engine.IntValue{5}}}},

	// 	// with datamodel
	// 	engine.And{engine.True{}, engine.And{engine.True{}, engine.Eq{engine.FloatValue{5}, engine.FieldValue{dm, "tx", []string{"amount"}}}}},
	// 	engine.And{engine.True{}, engine.And{engine.True{}, engine.Eq{engine.FloatValue{6}, engine.FieldValue{dm, "tx", []string{"amount"}}}}},
	// 	engine.And{engine.True{}, engine.And{engine.True{}, engine.Eq{engine.FloatValue{6}, engine.FieldValue{dm, "tx", []string{"sender"}}}}},
	// }

	// for _, e := range exprs {
	// 	fmt.Println(e.Print(p), " evals to :", e.Eval(p))
	// }

}
